package itemsync

import (
	"log"

	"github.com/rafaelespinoza/standardnotes/jobs"
	"github.com/rafaelespinoza/standardnotes/models"
)

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
