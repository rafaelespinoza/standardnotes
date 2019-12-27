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

type User struct {
	UUID      string    `json:"uuid"`
	Email     string    `json:"email"`
	Password  string    `json:"password,omitempty"`
	PwFunc    string    `json:"pw_func"     sql:"pw_func"`
	PwAlg     string    `json:"pw_alg"      sql:"pw_alg"`
	PwCost    int       `json:"pw_cost"     sql:"pw_cost"`
	PwKeySize int       `json:"pw_key_size" sql:"pw_key_size"`
	PwNonce   string    `json:"pw_nonce,omitempty"    sql:"pw_nonce"`
	PwAuth    string    `json:"pw_auth,omitempty"     sql:"pw_auth"`
	PwSalt    string    `json:"pw_salt,omitempty"     sql:"pw_salt"`
	CreatedAt time.Time `json:"created_at"  sql:"created_at"`
	UpdatedAt time.Time `json:"updated_at"  sql:"updated_at"`
}

// NewUser initializes a User with default values.
func NewUser() *User {
	user := User{}
	user.PwCost = 100000
	user.PwAlg = "sha512"
	user.PwKeySize = 512
	user.PwFunc = "pbkdf2"
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	return &user
}

var _ db.MigratingUser = (*User)(nil)

func (u *User) GetEmail() string   { return u.Email }
func (u *User) GetPwNonce() string { return u.PwNonce }
func (u *User) GetUUID() string    { return u.UUID }

// UpdatePassword updates the user's password.
func (u *User) UpdatePassword(np NewPassword) error {
	if u.UUID == "" {
		return fmt.Errorf("Unknown user")
	}

	u.Password = Hash(np.NewPassword)
	u.PwCost = np.PwCost
	u.PwSalt = np.PwSalt
	u.PwNonce = np.PwNonce

	u.UpdatedAt = time.Now()
	// TODO: validate incoming password params
	err := db.Query(`
		UPDATE users
		SET password=?, pw_cost=?, pw_salt=?, pw_nonce=?, updated_at=?
		WHERE uuid=?`,
		u.Password, u.PwCost, u.PwSalt, u.PwNonce, u.UpdatedAt,
		u.UUID,
	)

	if err != nil {
		logger.LogIfDebug(err)
		return err
	}

	return nil
}

// UpdateParams - update params
func (u *User) UpdateParams(p Params) error {
	if u.UUID == "" {
		return fmt.Errorf("Unknown user")
	}

	u.UpdatedAt = time.Now()
	err := db.Query(`
		UPDATE users
		SET pw_func=?, pw_alg=?, pw_cost=?, pw_key_size=?, pw_salt=?, updated_at=?
		WHERE uuid=?`,
		u.PwFunc, u.PwAlg, u.PwCost, u.PwKeySize, u.PwSalt, time.Now(),
		u.UUID,
	)

	if err != nil {
		logger.LogIfDebug(err)
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

// LoadByUUID populates the User's fields by querying the DB.
func (u *User) LoadByUUID(uuid string) (err error) {
	_, err = db.SelectStruct("SELECT * FROM users WHERE uuid=?", u, uuid)
	return
}

// Validate checks the jwt for a valid password.
func (u *User) Validate(password string) bool {
	return password == u.Password
}

// MakeSaferCopy duplicates the User value, but excludes some sensitive fields.
func (u User) MakeSaferCopy() User {
	u.Password = ""
	u.PwNonce = ""
	return u
}

// LoadByEmail populates the user fields with a DB lookup.
func (u *User) LoadByEmail(email string) error {
	_, err := db.SelectStruct("SELECT * FROM users WHERE email=?", u, email)
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
			pw_nonce, pw_auth, pw_salt, created_at, updated_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		u.UUID, u.Email, u.Password, u.PwFunc, u.PwAlg, u.PwCost, u.PwKeySize,
		u.PwNonce, u.PwAuth, u.PwSalt, u.CreatedAt, u.UpdatedAt)

	if err != nil {
		logger.LogIfDebug(err)
	}

	return err
}

func (u *User) LoadByEmailAndPassword(email, password string) (err error) {
	_, err = db.SelectStruct(
		"SELECT * FROM users WHERE email=? AND password=?",
		u, email, Hash(password),
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
		u.UUID, "SF|Extension", false,
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

// Params is the set of authentication parameters for the user.
type Params struct {
	PwFunc     string `json:"pw_func"     sql:"pw_func"`
	PwAlg      string `json:"pw_alg"      sql:"pw_alg"`
	PwCost     int    `json:"pw_cost"     sql:"pw_cost"`
	PwKeySize  int    `json:"pw_key_size" sql:"pw_key_size"`
	PwSalt     string `json:"pw_salt"     sql:"pw_salt"`
	PwNonce    string `json:"pw_nonce"    sql:"pw_nonce"`
	Version    string `json:"version"`
	Identifier string `json:"identifier"` // should be email address
}

// MakeAuthParams constructs authentication parameters from User fields. NOTE:
// it's tempting to put this into the interactors package, but you can't because
// you'd get an import cycle.
func MakeAuthParams(u User) Params {
	var params Params

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
