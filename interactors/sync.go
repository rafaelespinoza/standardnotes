package interactors

import (
	"encoding/json"
	"errors"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/rafaelespinoza/standardfile/jobs"
	"github.com/rafaelespinoza/standardfile/models"
)

// SyncRequest is a collection of named parameters for an incoming sync request.
type SyncRequest struct {
	Items            models.Items `json:"items"`
	SyncToken        string       `json:"sync_token"`
	CursorToken      string       `json:"cursor_token"`
	ContentType      string       `json:"content_type"`
	Limit            int          `json:"limit"`
	ComputeIntegrity bool         `json:"compute_integrity"`
}

// A SyncResponse is the output of an incoming sync request.
type SyncResponse struct {
	Retrieved     models.Items   `json:"retrieved_items"`
	Saved         models.Items   `json:"saved_items"`
	Conflicts     []ItemConflict `json:"conflicts"`
	SyncToken     string         `json:"sync_token"`
	CursorToken   string         `json:"cursor_token,omitempty"`
	IntegrityHash string         `json:"integrity_hash"`
}

// LoadValue - hydrate struct from map
func (r *SyncRequest) LoadValue(name string, value []string) { // TODO: rm this method
	switch name {
	case "items":
		r.Items = models.Items{}
	case "sync_token":
		r.SyncToken = value[0]
	case "cursor_token":
		r.CursorToken = value[0]
	case "limit":
		r.Limit, _ = strconv.Atoi(value[0])
	}
}

// SyncUserItems manages user item syncs.
func SyncUserItems(user models.User, req SyncRequest) (res *SyncResponse, err error) {
	var cursorTime time.Time

	if req.Limit < 1 {
		req.Limit = 100000
	}
	res = &SyncResponse{}

	if err = res.loadSyncItems(user, req); err != nil {
		return
	}
	if len(res.Retrieved) > 0 {
		cursorTime = res.Retrieved[len(res.Retrieved)-1].UpdatedAt
	}
	if !cursorTime.IsZero() {
		res.CursorToken = models.GetTokenFromTime(cursorTime)
	}

	if len(res.Saved) < 1 {
		res.SyncToken = models.GetTokenFromTime(time.Now())
	} else {
		res.SyncToken = models.GetTokenFromTime(res.Saved[0].UpdatedAt)
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

// loadSyncItems does a whole lot of stuff, but the TLDR is that it loads the
// user's items from the DB, compares those items against the incoming items
// from the request, then either creates new items or updates the existing
// items to the DB. Conflicting items cannot be saved to the DB, so they're
// collected in a separate list and sent back to the client.
func (r *SyncResponse) loadSyncItems(user models.User, req SyncRequest) (err error) {
	var retrieved models.Items
	var saved models.Items
	var conflicts []ItemConflict

	if retrieved, err = user.LoadItems(
		req.CursorToken,
		req.SyncToken,
		req.ContentType,
	); err != nil {
		return
	}
	r.Retrieved = retrieved

	for _, incomingItem := range req.Items {
		var item *models.Item
		// Probably don't need to go all the way back to the DB to check for
		// conflicts since the items ought to be retrieved by this point.
		// However, this may not be true if there's pagination. For now, just go
		// back to the DB until there's more knowledge.
		item, err = checkItemConflicts(incomingItem)
		if err == _ErrConflictingUUID {
			conflicts = append(conflicts, &uuidConflict{item: incomingItem})
			continue
		} else if err == _ErrConflictingSync {
			// Don't save the incoming value, add to the list of conflicted
			// items so the client doesn't try to resync it.
			conflicts = append(conflicts, &syncConflict{item: *item})
			r.Retrieved.Delete(item.UUID) // Exclude item from subsequent syncs.
			continue
		} else if err != nil {
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

	r.Saved = saved
	r.Conflicts = conflicts
	return
}

const _MinConflictInterval = 1 * time.Second

var (
	_ErrConflictingSync = errors.New("sync_conflict")
	// _ErrConflictingUUID signals a UUID conflict, this might happen if a user
	// is importing data from another account.
	_ErrConflictingUUID = errors.New("uuid_conflict")
)

// checkItemConflicts first checks if the incomingItem is in the DB. If it's not
// in the database, then it returns a pointer to the incoming Item. If it is,
// then it compares timestamps on the item found in the DB and the incomingItem.
// If they're the same, then assume both items are identical. If different
// (outside of a certain threshold), then consider it a sync conflict.
func checkItemConflicts(incomingItem models.Item) (item *models.Item, err error) {
	var alreadyExists bool
	if alreadyExists, err = incomingItem.Exists(); err != nil {
		// probably importing notes from another account? This is translated
		// from the ruby implementation, and I don't know how they decided that
		// any error here would be considered a conflicting UUID...
		err = _ErrConflictingUUID
		return
	} else if !alreadyExists {
		// hydrate item fields with incoming parameters
		item = &incomingItem
		return
	} else {
		// hydrate item fields with DB values
		if err = item.LoadByUUID(incomingItem.UUID); err != nil {
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
		// Probably stale data (when diff < 0). Or less likely, the data was
		// somehow manipulated (when diff > 0).
		saveIncoming = math.Abs(float64(diff)) < float64(_MinConflictInterval)
	}

	if !saveIncoming {
		err = _ErrConflictingSync
		return
	}

	return
}

type ItemConflict interface {
	Item() models.Item
	Conflict() error
}

type uuidConflict struct {
	item models.Item
}

var _ ItemConflict = (*uuidConflict)(nil)

func (c *uuidConflict) Item() models.Item { return c.item }
func (c *uuidConflict) Conflict() error   { return _ErrConflictingUUID }
func (c *uuidConflict) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"unsaved_item": c.Item(),
		"type":         c.Conflict().Error(),
	})
}

type syncConflict struct {
	item models.Item
}

var _ ItemConflict = (*syncConflict)(nil)

func (c *syncConflict) Item() models.Item { return c.item }
func (c *syncConflict) Conflict() error   { return _ErrConflictingSync }
func (c *syncConflict) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"server_item": c.Item(),
		"type":        c.Conflict().Error(),
	})
}

func enqueueRealtimeExtensionJobs(user models.User, items models.Items) (err error) {
	if len(items) < 1 {
		return
	}
	extensions, err := user.LoadActiveExtensionItems()
	if err != nil {
		return
	}
	for i, ext := range extensions {
		content := ext.DecodedContentMetadata()
		if content == nil || content.Frequency != models.FrequencyRealtime || len(content.URL) < 1 {
			continue
		}

		itemIDs := make([]string, len(items))
		for i, item := range items {
			itemIDs[i] = item.UUID
		}
		if err = jobs.PerformExtensionJob(
			jobs.ExtensionJobParams{
				URL:         content.URL,
				ItemIDs:     itemIDs,
				UserID:      user.UUID,
				ExtensionID: ext.UUID,
			},
		); err != nil {
			log.Printf(
				"could not perform job on extensions[%d]; %v",
				i, err,
			)
			return
		}
	}
	return
}

func enqueueDailyBackupExtensionJobs(items models.Items) (err error) {
	for i, item := range items {
		if !item.IsDailyBackupExtension() || item.Deleted {
			continue
		}
		content := item.DecodedContentMetadata()
		if content == nil {
			continue
		}

		if content.SubType == "backup.email_archive" {
			err = jobs.PerformMailerJob(
				jobs.MailerJobParams{UserID: item.UserUUID},
			)
		} else if content.Frequency == models.FrequencyDaily && content.URL != "" {
			err = jobs.PerformExtensionJob(
				jobs.ExtensionJobParams{
					URL:         content.URL,
					UserID:      item.UserUUID,
					ExtensionID: item.UUID,
				},
			)

		}
		if err != nil {
			log.Printf(
				"could not perform job on items[%d]; %v",
				i, err,
			)
		}
	}
	return
}
