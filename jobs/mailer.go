package jobs

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/rafaelespinoza/standardfile/db"
	"github.com/rafaelespinoza/standardfile/models"
)

type MailerJobParams struct {
	UserID string
}

func PerformMailerJob(params MailerJobParams) (err error) {
	// TODO: send user an email with an attachment, a JSON file listing all
	// undeleted items.
	var user models.User
	var contents struct {
		Items      models.Items
		AuthParams models.Params
	}
	var attachment struct {
		Filename string
		MimeType string
		Content  []byte
	}

	if err = db.Select(`
		SELECT email
		FROM 'users'
		WHERE 'user_uuid'=?
		LIMIT 1`,
		&user,
		params.UserID,
	); err != nil {
		return
	}
	if err = db.Select(`
		SELECT * FROM items
		WHERE user_uuid = ? AND deleted IS NULL
		ORDER BY 'updated_at' DESC`,
		&contents.Items,
	); err != nil {
		return
	}
	if data, perr := json.Marshal(contents); perr != nil {
		err = perr
		return
	} else {
		attachment.Content = data
	}

	contents.AuthParams = user.AuthParams()
	attachment.Filename = fmt.Sprintf("SN-Data-%s.txt", time.Now().Format("20060102015405"))
	attachment.MimeType = "application/json"
	log.Printf("TODO: send email to user with attachment\n")
	return
}
