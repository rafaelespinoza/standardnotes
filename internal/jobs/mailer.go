package jobs

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/rafaelespinoza/standardnotes/internal/models"
)

type MailerJobParams struct {
	UserID string
}

func PerformMailerJob(params MailerJobParams) (err error) {
	var user *models.User
	var contents struct {
		Items      models.Items
		AuthParams models.PwGenParams
	}
	var attachment struct {
		Filename string
		MimeType string
		Content  []byte
	}

	if user, err = models.LoadUserByUUID(params.UserID); err != nil {
		return
	}
	if items, ierr := user.LoadActiveItems(); ierr != nil {
		err = ierr
		return
	} else {
		contents.Items = items
	}
	if data, perr := json.Marshal(contents); perr != nil {
		err = perr
		return
	} else {
		attachment.Content = data
	}

	contents.AuthParams = models.MakePwGenParams(*user)
	attachment.Filename = fmt.Sprintf("SN-Data-%s.txt", time.Now().Format("20060102015405"))
	attachment.MimeType = "application/json"
	// TODO: send email to user with attached JSON file.
	return
}
