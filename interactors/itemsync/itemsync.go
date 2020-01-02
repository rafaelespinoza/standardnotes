package itemsync

import (
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/rafaelespinoza/standardfile/logger"
	"github.com/rafaelespinoza/standardfile/models"
)

// Request is a collection of named parameters for an incoming sync request.
type Request struct {
	Items            models.Items `json:"items"`
	SyncToken        string       `json:"sync_token"`
	CursorToken      string       `json:"cursor_token"`
	ContentType      string       `json:"content_type"`
	Limit            int          `json:"limit"`
	ComputeIntegrity bool         `json:"compute_integrity"`
}

// A Response is the output of an incoming sync request.
type Response struct {
	Retrieved     models.Items   `json:"retrieved_items"`
	Saved         models.Items   `json:"saved_items"`
	Conflicts     []ItemConflict `json:"conflicts"`
	SyncToken     string         `json:"sync_token"`
	CursorToken   string         `json:"cursor_token,omitempty"`
	IntegrityHash string         `json:"integrity_hash"`
}

// SyncUserItems manages user item syncs.
func SyncUserItems(user models.User, req Request) (res *Response, err error) {
	res = &Response{
		Retrieved: make([]models.Item, 0),
		Saved:     make([]models.Item, 0),
		Conflicts: make([]ItemConflict, 0),
	}

	if len(user.UUID) < models.MinIDLength {
		err = itemSyncError{
			error:      fmt.Errorf("user id invalid"),
			validation: true,
		}
		return
	}

	if err = res.doItemSync(user, req); err != nil {
		return
	}

	err = enqueueRealtimeExtensionJobs(user, req.Items)
	if err != nil {
		return
	}
	if err = enqueueDailyBackupExtensionJobs(res.Saved); err != nil {
		return
	}

	if !req.ComputeIntegrity {
		return
	}
	userItems, err := user.LoadActiveItems()
	if err != nil {
		return
	}
	res.IntegrityHash = models.Items(userItems).ComputeHashDigest()
	return
}

// doItemSync does a whole lot of stuff, but the TLDR is that it loads the user
// items from the DB, compares the items against the incoming items from the
// request, then either creates new items or updates the existing items to the
// DB. Conflicting items cannot be saved to the DB, so they're collected in a
// separate list and sent back to the client.
func (r *Response) doItemSync(user models.User, req Request) (err error) {
	var retrieved models.Items
	var saved models.Items
	var conflicts []ItemConflict

	// prepare a sync by loading the user's items from the DB.
	limit := req.Limit
	if limit <= 1 {
		limit = models.UserItemMaxPageSize / 2
	} else if limit > models.UserItemMaxPageSize {
		limit = models.UserItemMaxPageSize
	}

	if req.CursorToken != "" {
		date := decodePaginationToken(req.CursorToken)
		retrieved, err = user.LoadItemsAfter(date, true, req.ContentType, limit)
	} else if req.SyncToken != "" {
		date := decodePaginationToken(req.SyncToken)
		retrieved, err = user.LoadItemsAfter(date, false, req.ContentType, limit)
	} else {
		retrieved, err = user.LoadAllItems(req.ContentType, limit)
	}

	if err != nil {
		return
	}

	// sync user items, identify conflicts.
	for _, incomingItem := range req.Items {
		var item *models.Item
		// ierr is the error is scoped to this block. You don't want a
		// conflicting item error to overwrite this function's return error.
		var ierr error
		// Probably don't need to go all the way back to the DB to check for
		// conflicts since the items ought to be retrieved by this point.
		// However, this may not be true if there's pagination. For now, just go
		// back to the DB until there's more knowledge.
		item, ierr = findCheckItem(incomingItem)
		if ierr == errUUIDConflict {
			conflicts = append(conflicts, &uuidConflict{item: incomingItem})
			continue
		} else if ierr == errSyncConflict {
			// Don't save the incoming value, add to the list of conflicted
			// items so the client doesn't try to resync it.
			conflicts = append(conflicts, &syncConflict{item: *item})
			retrieved.Delete(item.UUID) // Exclude item from subsequent syncs.
			continue
		} else if ierr != nil {
			err = ierr
			return
		}
		// in case the incoming item tries to change UserUUID, change it back to
		// the known user.
		item.UserUUID = user.UUID
		incomingItem.UserUUID = user.UUID
		if err = item.MergeProtected(&incomingItem); err != nil {
			return
		}

		// Can *probably* do Save or Delete instead of potentially doing both.
		// But before doing that, consider if there are other things that need
		// to be saved before it's marked as "deleted".
		if err = item.Save(); err != nil {
			return
		}
		if item.Deleted {
			if err = item.Delete(); err != nil {
				return
			}
		}
		saved = append(saved, *item)
	}
	if saved == nil {
		saved = make([]models.Item, 0)
	}
	if conflicts == nil {
		conflicts = make([]ItemConflict, 0)
	}
	if len(retrieved) >= limit { // could there be more rows?
		// Should be greatest value. Depends on the ordering of DB results.
		cursorTime := retrieved[len(retrieved)-1].UpdatedAt
		r.CursorToken = encodePaginationToken(cursorTime)
	}

	var latestUpdate time.Time
	if len(saved) > 0 {
		// Should be greatest value. Depends on the ordering of DB results.
		latestUpdate = saved[len(saved)-1].UpdatedAt
	} else {
		latestUpdate = time.Now()
	}
	// avoid returning same row in a subsequent sync
	latestUpdate = latestUpdate.Add(time.Microsecond)
	r.SyncToken = encodePaginationToken(latestUpdate)

	r.Retrieved = retrieved
	r.Saved = saved
	r.Conflicts = conflicts

	return
}

// findCheckItem first looks for the incomingItem in the DB. If it's not in the
// database, then it returns a pointer to the incoming Item. If it is, then it
// compares timestamps on the item found in the DB and the incomingItem. If
// they're the same, then assume both items are identical. If different (outside
// of a certain threshold), then consider it a sync conflict.
func findCheckItem(incomingItem models.Item) (item *models.Item, err error) {
	var alreadyExists bool
	if alreadyExists, err = incomingItem.Exists(); err != nil {
		// probably importing notes from another account? This is translated
		// from the ruby implementation, and I don't know how they decided that
		// any error here would be considered a conflicting UUID...
		err = errUUIDConflict
		return
	} else if !alreadyExists {
		// hydrate item fields with incoming parameters
		item = &incomingItem
		return
	} else {
		// hydrate item fields with DB values
		if item, err = models.LoadItemByUUID(incomingItem.UUID); err != nil {
			return
		}
	}

	// By this point, we know the item exists in the DB. Time to look more into
	// it and decide if it's a conflict.

	var saveIncoming bool
	theirsUpdated := incomingItem.UpdatedAt
	oursUpdated := item.UpdatedAt
	diff := theirsUpdated.Sub(oursUpdated) * time.Second

	if diff == 0 {
		saveIncoming = true
	} else {
		// If diff was < 0, it's probably stale data. Or less likely, if diff
		// was > 0, then the data was probably manipulated somehow.
		saveIncoming = math.Abs(float64(diff)) < float64(_MinConflictThreshold)
	}

	if !saveIncoming {
		err = errSyncConflict
		return
	}

	return
}

const _TokenVersion = "2"

// encodePaginationToken generates a token for the time.
func encodePaginationToken(date time.Time) string {
	return base64.URLEncoding.EncodeToString(
		[]byte(
			fmt.Sprintf(
				"%s:%d",
				_TokenVersion, date.UnixNano(),
			),
		),
	)
}

// decodePaginationToken converts a token to a time.
func decodePaginationToken(token string) time.Time {
	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		logger.LogIfDebug(err)
		return time.Now()
	}
	parts := strings.Split(string(decoded), ":")
	if len(parts) != 2 {
		err = fmt.Errorf("expected %d parts in decoded token", 2)
		logger.LogIfDebug(err)
		return time.Now()
	}
	num, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		logger.LogIfDebug(err)
		return time.Now()
	}
	return time.Unix(0, num)
}

type itemSyncError struct {
	error
	notFound   bool
	validation bool
}

func (i itemSyncError) NotFound() bool   { return i.notFound }
func (i itemSyncError) Validation() bool { return i.validation }
