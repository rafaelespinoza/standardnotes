package models

import (
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kisielk/sqlstruct"
	"github.com/rafaelespinoza/standardfile/db"
	"github.com/rafaelespinoza/standardfile/encryption"
	"github.com/rafaelespinoza/standardfile/logger"
	uuid "github.com/satori/go.uuid"
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

// LoadValue hydrates a struct from a map.
func (u *User) LoadValue(name string, value []string) {
	switch name {
	case "uuid":
		u.UUID = value[0]
	case "email":
		u.Email = value[0]
	case "password":
		u.Password = value[0]
	case "pw_func":
		u.PwFunc = value[0]
	case "pw_alg":
		u.PwAlg = value[0]
	case "pw_auth":
		u.PwAuth = value[0]
	case "pw_salt":
		u.PwSalt = value[0]
	case "pw_cost":
		u.PwCost, _ = strconv.Atoi(value[0])
	case "pw_key_size":
		u.PwKeySize, _ = strconv.Atoi(value[0])
	case "pw_nonce":
		u.PwNonce = value[0]
	}
}

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
		UPDATE 'users'
		SET 'password'=?, 'pw_cost'=?, 'pw_salt'=?, 'pw_nonce'=?, 'updated_at'=?
		WHERE 'uuid'=?`,
		u.Password, u.PwCost, u.PwSalt, u.PwNonce, u.UpdatedAt,
		u.UUID,
	)

	if err != nil {
		logger.Log(err)
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
		UPDATE 'users'
		SET 'pw_func'=?, 'pw_alg'=?, 'pw_cost'=?, 'pw_key_size'=?, 'pw_salt'=?, 'updated_at'=?
		WHERE 'uuid'=?`,
		u.PwFunc, u.PwAlg, u.PwCost, u.PwKeySize, u.PwSalt, time.Now(),
		u.UUID,
	)

	if err != nil {
		logger.Log(err)
		return err
	}

	return nil
}

// Exists checks if the user exists in the DB.
func (u User) Exists() bool {
	uuid, err := db.SelectFirst(
		"SELECT 'uuid' FROM 'users' WHERE 'email'=?",
		u.Email,
	)

	if err != nil {
		logger.Log(err)
		return false
	}

	return uuid != ""
}

// LoadByUUID hydrates the user from the DB.
func (u *User) LoadByUUID(uuid string) bool {
	_, err := db.SelectStruct(
		fmt.Sprintf(
			"SELECT %s FROM 'users' WHERE 'uuid'=?",
			sqlstruct.Columns(User{}),
		),
		u, uuid,
	)
	if err != nil {
		logger.Log("Load err:", err)
		return false
	}

	return true
}

// Validate checks the jwt for a valid password.
func (u User) Validate(password string) bool {
	return password == u.Password
}

// ToJSON - return map without pw and nonce
func (u User) ToJSON() interface{} { // TODO: rm this method
	u.Password = ""
	u.PwNonce = ""
	return u
}

// LoadByEmail populates the user fields with a DB lookup.
func (u *User) LoadByEmail(email string) error {
	_, err := db.SelectStruct("SELECT * FROM 'users' WHERE 'email'=?", u, email)
	if err != nil {
		logger.Log(err)
	}
	return err
}

// Create saves the user to the DB.
func (u *User) Create() error {
	if u.UUID != "" {
		return fmt.Errorf("Trying to save existing user")
	}

	if u.Email == "" || u.Password == "" {
		return fmt.Errorf("Empty email or password")
	}

	if u.Exists() {
		return fmt.Errorf("Unable to register")
	}

	id := uuid.NewV4()
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
		logger.Log(err)
	}

	return err
}

func (u *User) LoadByEmailAndPassword(email, password string) {
	_, err := db.SelectStruct(
		"SELECT * FROM 'users' WHERE 'email'=? AND 'password'=?",
		u, email, Hash(password),
	)
	if err != nil {
		logger.Log(err)
	}
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

	if u.PwFunc != "" {
		params.PwFunc = u.PwFunc
		params.PwAlg = u.PwAlg
		params.PwKeySize = u.PwKeySize
	}

	if u.PwNonce != "" {
		params.PwNonce = u.PwNonce
	}

	if u.PwSalt == "" {
		nonce := u.PwNonce
		if nonce == "" {
			nonce = "a04a8fe6bcb19ba61c5c0873d391e987982fbbd4"
		}
		u.PwSalt = encryption.Salt(u.Email, nonce)
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

func (u User) LoadActiveItems() (items Items, err error) {
	err = db.Select(`
		SELECT * FROM 'items'
		WHERE 'user_uuid'=? AND 'content_type' IS NOT '' AND deleted = ?
		ORDER BY 'updated_at' DESC`,
		&items,
		u.UUID, "SF|Extension", false,
	)
	return
}

func (u User) LoadActiveExtensionItems() (items Items, err error) {
	err = db.Select(`
		SELECT * FROM 'items'
		WHERE 'user_uuid'=? AND 'content_type' = ? AND deleted = ?
		ORDER BY 'updated_at' DESC`,
		&items,
		u.UUID, "SF|Extension", false,
	)
	return
}

func (u User) LoadItems(request SyncRequest) (items Items, cursorTime time.Time, err error) {
	if request.CursorToken != "" {
		logger.Log("loadItemsFromDate")
		items, err = u.loadItemsFromDate(GetTimeFromToken(request.CursorToken))
	} else if request.SyncToken != "" {
		logger.Log("loadItemsOlder")
		items, err = u.loadItemsOlder(GetTimeFromToken(request.SyncToken))
	} else {
		logger.Log("loadItems")
		items, err = u.loadAllItems(request.Limit)
		if len(items) > 0 {
			cursorTime = items[len(items)-1].UpdatedAt
		}
	}
	return items, cursorTime, err
}

func (u User) loadItemsFromDate(date time.Time) (items Items, err error) {
	err = db.Select(`
		SELECT *
		FROM 'items'
		WHERE 'user_uuid'=? AND 'updated_at' >= ?
		ORDER BY 'updated_at' DESC`,
		&items, u.UUID, date,
	)
	return
}

func (u User) loadItemsOlder(date time.Time) (items Items, err error) {
	err = db.Select(`
		SELECT *
		FROM 'items'
		WHERE 'user_uuid'=? AND 'updated_at' > ?
		ORDER BY 'updated_at' DESC`,
		&items, u.UUID, date,
	)
	return
}

func (u User) loadAllItems(limit int) (items Items, err error) {
	err = db.Select(
		"SELECT * FROM 'items' WHERE 'user_uuid'=? ORDER BY 'updated_at' DESC",
		&items, u.UUID,
	)
	return
}
