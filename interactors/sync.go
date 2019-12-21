package interactors

import (
	"fmt"
	"log"
	"math"
	"time"

	mapset "github.com/deckarep/golang-set"
	"github.com/rafaelespinoza/standardfile/jobs"
	"github.com/rafaelespinoza/standardfile/logger"
	"github.com/rafaelespinoza/standardfile/models"
)

// SyncUserItems manages user item syncs.
func SyncUserItems(user models.User, req models.SyncRequest) (res *models.SyncResponse, err error) {
	if res, err = syncUserItems(user, req); err != nil {
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

func syncUserItems(user models.User, req models.SyncRequest) (res *models.SyncResponse, err error) {
	res = &models.SyncResponse{
		Retrieved:   models.Items{},
		Saved:       models.Items{},
		Unsaved:     []models.Unsaved{},
		SyncToken:   models.GetTokenFromTime(time.Now()),
		CursorToken: "",
	}

	if req.Limit < 1 {
		req.Limit = 100000
	}
	var cursorTime time.Time

	logger.Log("Load items")
	res.Retrieved, cursorTime, err = user.LoadItems(req)

	if err != nil {
		return res, err
	}
	if !cursorTime.IsZero() {
		res.CursorToken = models.GetTokenFromTime(cursorTime)
	}
	logger.Log("Save incoming items", req)
	res.Saved, res.Unsaved, err = req.Items.Save(user.UUID)
	if err != nil {
		return res, err
	}
	if len(res.Saved) > 0 {
		res.SyncToken = models.GetTokenFromTime(res.Saved[0].UpdatedAt)
		logger.Log("Conflicts check")
		checkConflicts(res.Saved, &res.Retrieved)
	}
	return
}

type itemSyncResult struct {
	saved     []models.Item
	conflicts []conflictedItem
}

type itemConflict uint8

const (
	_ItemConflictUnknown itemConflict = iota
	_ItemConflictUUID
	_ItemConflictSync
)

func (c itemConflict) String() string {
	return [...]string{"unknown_conflict", "uuid_conflict", "sync_conflict"}[c]
}

type conflictedItem interface {
	Type() itemConflict
}

type uuidConflict struct {
	UnsavedItem models.Item `json:"unsaved_item"`
}

var _ conflictedItem = (*uuidConflict)(nil)

func (c *uuidConflict) Type() itemConflict { return _ItemConflictUUID }

type syncConflict struct {
	ServerItem models.Item `json:"server_item"`
}

var _ conflictedItem = (*syncConflict)(nil)

func (c *syncConflict) Type() itemConflict { return _ItemConflictSync }

// saveUserItems is a remake of a method in the Rails syncing-server, at:
// https://github.com/standardnotes/syncing-server/blob/master/lib/sync_engine/2019_05_20/sync_manager.rb#L37
func saveUserItems(user models.User, req models.SyncRequest, retrievedItems models.Items) (out *itemSyncResult, err error) {
	saved := make([]models.Item, 0)
	conflicts := make([]conflictedItem, 0)
	for _, incomingItem := range req.Items {
		var alreadyExists bool
		var item *models.Item // will either be an existing item or new item (incomingItem)
		if alreadyExists, err = incomingItem.Exists(); err != nil {
			conflicts = append(conflicts, &uuidConflict{UnsavedItem: incomingItem})
			continue
		} else if alreadyExists {
			// hydrate item fields with DB values
			if err = item.LoadByUUID(incomingItem.UUID); err != nil {
				return
			}
		} else {
			// hydrate item fields incoming parameters
			item = &incomingItem
		}
		incomingUpdatedAt := incomingItem.UpdatedAt

		if alreadyExists {
			var saveIncoming bool
			oursUpdatedAt := incomingItem.UpdatedAt
			if incomingUpdatedAt.Before(oursUpdatedAt) {
				// probably stale data
				saveIncoming = incomingUpdatedAt.Sub(oursUpdatedAt) < _MinConflictInterval
			} else if oursUpdatedAt.Before(incomingUpdatedAt) {
				saveIncoming = incomingUpdatedAt.Sub(oursUpdatedAt) < _MinConflictInterval
			} else {
				saveIncoming = true
			}

			if !saveIncoming {
				// don't save the incoming value, don't send it back to client.
				// The item found by the server is likely the same as an item in
				// retrievedItems. To prevent it from being included in a
				// subsequent sync, set up serialization of the in-memory copy.
				conflicts = append(conflicts, &syncConflict{ServerItem: *item})
				retrievedItems.Delete(item.UUID)
				continue
			}
		}

		if err = item.Update(); err != nil {
			return
		}

		if item.Deleted {
			if err = item.Delete(); err != nil {
				return
			}
		}
		saved = append(saved, *item)
	}
	out = &itemSyncResult{
		saved:     saved,
		conflicts: conflicts,
	}
	return
}

func enqueueRealtimeExtensionJobs(user models.User, items models.Items) (err error) {
	if len(items) < 1 {
		return
	}
	extensions, err := user.LoadActiveExtensionItems()
	if err != nil {
		return
	}
	for _, ext := range extensions {
		content := ext.DecodedContentMetadata()
		if content == nil || content.Frequency != models.FrequencyRealtime || len(content.URL) < 1 {
			continue
		}

		itemIDs := make([]string, len(items))
		for i, item := range items {
			itemIDs[i] = item.UUID
		}
		err = jobs.PerformExtensionJob(
			jobs.ExtensionJobParams{
				URL:         content.URL,
				ItemIDs:     itemIDs,
				UserID:      user.UUID,
				ExtensionID: ext.UUID,
			},
		)
		if err != nil {
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

const _MinConflictInterval = 20.0

func checkConflicts(items models.Items, existing *models.Items) {
	logger.Log(fmt.Sprintf("saved len: %d, retrieved len: %d", len(items), len(*existing)))
	saved := mapset.NewSet()
	for _, item := range items {
		saved.Add(item.UUID)
	}
	retrieved := mapset.NewSet()
	for _, item := range *existing {
		retrieved.Add(item.UUID)
	}
	conflicts := saved.Intersect(retrieved)
	logger.Log("conflicts", conflicts)

	// saved items take precedence, retrieved items are duplicated with new uuid
	for _, uuid := range conflicts.ToSlice() {
		// if changes are greater than _MinConflictInterval seconds apart, create
		// conflicted copy, otherwise discard conflicted
		savedCopy := items.Find(uuid.(string))
		retrievedCopy := existing.Find(uuid.(string))

		diff := math.Abs(float64(
			savedCopy.UpdatedAt.Unix() - retrievedCopy.UpdatedAt.Unix(),
		))
		logger.Log(fmt.Sprintf(
			"conflicted diff: %f, limit: %f", diff, _MinConflictInterval,
		))
		if diff > _MinConflictInterval { // is there a conflict?
			log.Printf("Creating conflicted copy of %v\n", uuid)
			dupe, err := retrievedCopy.Copy()
			if err != nil {
				logger.Log(err)
			} else {
				*existing = append(*existing, dupe)
			}
		}
		existing.Delete(uuid.(string))
	}
}
