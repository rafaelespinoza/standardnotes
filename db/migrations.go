package db

import (
	"database/sql"
	"log"
	"time"

	"github.com/rafaelespinoza/standardfile/config"
	"github.com/rafaelespinoza/standardfile/encryption"
	m "github.com/remind101/migrate"
)

// MigratingUser facilitates database migrations with a User.
type MigratingUser interface {
	GetEmail() string
	GetPwNonce() string
	GetUUID() string
}

// Migrate performs migration
func Migrate(cfg config.Config) {
	Init(cfg.DB)
	migrations := getMigrations()
	err := m.Exec(DB(), m.Up, migrations...)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Done")
}

func getMigrations() []m.Migration {
	migrations := []m.Migration{
		{
			ID: 1,
			Up: m.Queries([]string{
				"ALTER TABLE users ADD COLUMN pw_salt varchar(255);",
			}),
			Down: func(tx *sql.Tx) error {
				// It's not possible to remove a column with sqlite.
				return nil
			},
		},
		{
			ID: 2,
			Up: func(tx *sql.Tx) error {
				users := []MigratingUser{}
				Select("SELECT * FROM `users`", &users)
				log.Println("Got", len(users), "users to update")
				for _, u := range users {
					email := u.GetEmail()
					nonce := u.GetPwNonce()
					if email == "" || nonce == "" {
						continue
					}
					if _, err := tx.Exec("UPDATE `users` SET `pw_salt`=?, `updated_at`=? WHERE `uuid`=?", encryption.Salt(email, nonce), time.Now(), u.GetUUID()); err != nil {
						log.Println(err)
					}
				}

				return nil
			},
			Down: func(tx *sql.Tx) error {
				return nil
			},
		},
	}
	return migrations
}
