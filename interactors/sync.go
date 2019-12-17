package interactors

import (
	"log"
	"time"

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
		res.Saved.CheckForConflicts(&res.Retrieved)
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
