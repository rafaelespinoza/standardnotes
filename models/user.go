package models

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rafaelespinoza/standardfile/db"
	"github.com/rafaelespinoza/standardfile/encryption"
	"github.com/rafaelespinoza/standardfile/logger"
)

// A User is the application's end user.
type User struct {
	UUID      string    `json:"uuid"`
	Email     string    `json:"email"`
	Password  string    `json:"password,omitempty"`
	PwFunc    string    `json:"pw_func"     sql:"pw_func"`
	PwAlg     string    `json:"pw_alg"      sql:"pw_alg"`
	PwCost    int       `json:"pw_cost"     sql:"pw_cost"`
	PwKeySize int       `json:"pw_key_size" sql:"pw_key_size"`
	PwNonce   string    `json:"pw_nonce,omitempty"    sql:"pw_nonce"`
	PwSalt    string    `json:"pw_salt,omitempty"     sql:"pw_salt"`
	CreatedAt time.Time `json:"created_at"  sql:"created_at"`
	UpdatedAt time.Time `json:"updated_at"  sql:"updated_at"`
}

// NewUser initializes a User with default values.
func NewUser() *User {
	return &User{
		PwCost:    100000,
		PwAlg:     "sha512",
		PwKeySize: 512,
		PwFunc:    "pbkdf2",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

var _ db.MigratingUser = (*User)(nil)

func (u *User) GetEmail() string   { return u.Email }
func (u *User) GetPwNonce() string { return u.PwNonce }
func (u *User) GetUUID() string    { return u.UUID }

// LoadUserByUUID fetches a User from the DB.
func LoadUserByUUID(uuid string) (user *User, err error) {
	if uuid == "" {
		err = ErrEmptyUUID
		return
	}
	user = NewUser() // can't be nil to start out
	err = db.SelectStruct(
		`SELECT * FROM users WHERE uuid = ?`,
		user,
		uuid,
	)
	if err != nil {
		user = nil
	}
	return
}

// Update performs a db update on the User.
func (u *User) Update(updates User) (err error) {
	if u.UUID == "" {
		return fmt.Errorf("Unknown user")
	}
	dupe := u.makeUnsafeCopy() // in case of db error, rollback in-memory.

	u.Password = updates.Password
	u.PwAlg = updates.PwAlg
	u.PwCost = updates.PwCost
	u.PwFunc = updates.PwFunc
	u.PwKeySize = updates.PwKeySize
	u.PwNonce = updates.PwNonce
	u.PwSalt = updates.PwSalt
	u.UpdatedAt = time.Now()

	err = db.Query(`
		UPDATE users
		SET password=?, pw_alg=?, pw_cost=?, pw_func=?, pw_key_size=?, pw_nonce=?, pw_salt=?, updated_at=?
		WHERE uuid=?`,
		u.Password, u.PwAlg, u.PwCost, u.PwFunc, u.PwKeySize, u.PwNonce, u.PwSalt, u.UpdatedAt,
		u.UUID,
	)

	if err != nil {
		logger.LogIfDebug(err)
		u = &dupe
		return err
	}

	return nil
}

// Exists checks if the user exists in the DB.
func (u *User) Exists() (bool, error) {
	if u.UUID == "" {
		return false, nil
	}
	return db.SelectExists("SELECT uuid FROM users WHERE email=?", u.Email)
}

// Validate checks the jwt for a valid password.
func (u *User) Validate(password string) bool {
	return password == u.Password
}

// MakeSaferCopy duplicates the User value, but excludes some sensitive fields.
func (u User) MakeSaferCopy() User {
	return u.duplicate(true)
}

// makeUnsafeCopy returns a duplicate User, including the sensitive fields.
func (u User) makeUnsafeCopy() User {
	return u.duplicate(false)
}

// duplicate returns a deep copy of the User. As it's currently implemented, it
// relies on value receiver semantics to copy the fields. So if any User fields
// become pointers or any kind of "reference type", such as map, slice, channel,
// the way it's implemented could lead to memory leaks.
func (u User) duplicate(includeSensitive bool) User {
	if !includeSensitive {
		return u
	}

	u.Password = ""
	u.PwNonce = ""
	return u
}

// LoadByEmail populates the user fields with a DB lookup.
func (u *User) LoadByEmail(email string) error {
	err := db.SelectStruct("SELECT * FROM users WHERE email=?", u, email)
	if err != nil {
		logger.LogIfDebug(err)
	}
	return err
}

// Create saves the user to the DB.
func (u *User) Create() error {
	if u.UUID != "" {
		return fmt.Errorf("cannot recreate existing user")
	}

	if u.Email == "" || u.Password == "" {
		return fmt.Errorf("empty email or password")
	}

	if exists, err := u.Exists(); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("unable to register; already exists")
	}

	id := uuid.New()
	u.UUID = uuid.Must(id, nil).String()
	u.Password = Hash(u.Password)
	u.CreatedAt = time.Now()

	err := db.Query(`
		INSERT INTO users (
			uuid, email, password, pw_func, pw_alg, pw_cost, pw_key_size,
			pw_nonce, pw_salt, created_at, updated_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		u.UUID, u.Email, u.Password, u.PwFunc, u.PwAlg, u.PwCost, u.PwKeySize,
		u.PwNonce, u.PwSalt, u.CreatedAt, u.UpdatedAt)

	if err != nil {
		logger.LogIfDebug(err)
	}

	return err
}

// LoadByEmailAndPassword populates user fields by looking up the email and
// hashed password. The password argument should already be hashed.
func (u *User) LoadByEmailAndPassword(email, password string) (err error) {
	err = db.SelectStruct(
		"SELECT * FROM users WHERE email=? AND password=?",
		u, email, password,
	)
	if err != nil {
		logger.LogIfDebug(err)
	}
	return
}

func (u *User) LoadActiveItems() (items Items, err error) {
	err = db.Select(`
		SELECT * FROM items
		WHERE user_uuid=? AND content_type IS NOT '' AND deleted = ?
		ORDER BY updated_at DESC`,
		&items,
		u.UUID, false,
	)
	return
}

func (u *User) LoadActiveExtensionItems() (items Items, err error) {
	err = db.Select(`
		SELECT * FROM items
		WHERE user_uuid=? AND content_type = ? AND deleted = ?
		ORDER BY updated_at DESC`,
		&items,
		u.UUID, "SF|Extension", false,
	)
	return
}

// UserItemMaxPageSize is the maximum amount of user items to return in a query.
const UserItemMaxPageSize = 1000

// LoadItemsAfter fetches user items from the DB. If gte is true,  then it
// performs a >= comparison on the updated at field. Otherwise, it does a >
// comparison.
func (u *User) LoadItemsAfter(date time.Time, gte bool, contentType string, limit int) (items Items, err error) {
	// TODO: add condition: `WHERE content_type = req.ContentType`
	// TODO: use limit, set to max if too high.
	if gte {
		err = db.Select(`
			SELECT *
			FROM items
			WHERE user_uuid=? AND updated_at >= ?
			ORDER BY updated_at DESC`,
			&items, u.UUID, date,
		)
	} else {
		err = db.Select(`
			SELECT *
			FROM items
			WHERE user_uuid=? AND updated_at > ?
			ORDER BY updated_at DESC`,
			&items, u.UUID, date,
		)

	}
	return
}

// LoadAllItems fetches all the user's items up to limit. Typically, this is
// used for initial item syncs.
func (u *User) LoadAllItems(contentType string, limit int) (items Items, err error) {
	// TODO: add condition: `WHERE content_type = req.ContentType`
	// TODO: use limit, set to max if too high.
	err = db.Select(
		"SELECT * FROM items WHERE user_uuid=? AND deleted = ? ORDER BY updated_at DESC",
		&items, u.UUID, false,
	)
	return
}

// PwGenParams is a set of authentication parameters used by the client to
// generate user passwords.
type PwGenParams struct {
	PwFunc     string `json:"pw_func"`
	PwAlg      string `json:"pw_alg"`
	PwCost     int    `json:"pw_cost"`
	PwKeySize  int    `json:"pw_key_size"`
	PwSalt     string `json:"pw_salt"`
	PwNonce    string `json:"pw_nonce"`
	Version    string `json:"version"`
	Identifier string `json:"identifier"` // should be email address
}

// MakePwGenParams constructs authentication parameters from User fields. NOTE:
// it's tempting to put this into the interactors package, but you can't because
// you'd get an import cycle.
func MakePwGenParams(u User) PwGenParams {
	var params PwGenParams

	if u.Email == "" {
		return params
	}

	params.Version = "003"
	params.PwCost = u.PwCost
	params.Identifier = u.Email

	if u.PwFunc != "" { // v1 only
		params.PwFunc = u.PwFunc
		params.PwAlg = u.PwAlg
		params.PwKeySize = u.PwKeySize
	}
	if u.PwSalt == "" { // v2 only
		nonce := u.PwNonce
		if nonce == "" {
			nonce = "a04a8fe6bcb19ba61c5c0873d391e987982fbbd4"
		}
		u.PwSalt = encryption.Salt(u.Email, nonce)
	}
	if u.PwNonce != "" { // v3 only
		params.PwNonce = u.PwNonce
	}

	params.PwSalt = u.PwSalt

	return params
}

// NewPassword helps facilitate user password changes.
type NewPassword struct {
	User
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// Hash computes a sha256 checksum of the input.
func Hash(input string) string {
	return strings.Replace(
		fmt.Sprintf("% x", sha256.Sum256([]byte(input))),
		" ",
		"",
		-1,
	)
}
