package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rafaelespinoza/standardfile/db"
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

	// passwordHashed tracks the hashing state for the Password field.
	passwordHashed bool
}

// NewUser initializes a User with default values.
func NewUser() *User {
	return &User{
		PwCost:    110000,
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
		err = validationError{fmt.Errorf("uuid is empty")}
		return
	}
	user = NewUser() // can't be nil to start out
	if err = user.fetchHydrate(`SELECT * FROM users WHERE uuid = ?`, uuid); err != nil {
		user = nil
	}
	return
}

// LoadByEmail populates the user fields with a DB lookup.
func LoadUserByEmail(email string) (user *User, err error) {
	if verr := ValidateEmail(email); verr != nil {
		err = verr
		return
	}
	user = NewUser()
	if err = user.fetchHydrate("SELECT * FROM users WHERE email=?", email); err != nil {
		user = nil
	}
	return
}

// LoadUserByEmailAndPassword populates user fields by looking up the user email
// and hashed password.
func LoadUserByEmailAndPassword(email, password string) (user *User, err error) {
	if err = ValidateEmail(email); err != nil {
		return
	} else if err = ValidatePassword(password); err != nil {
		return
	}
	user = NewUser()
	if err = user.fetchHydrate(
		"SELECT * FROM users WHERE email=? AND password=?",
		email, password,
	); err != nil {
		user = nil
	}
	return
}

func (u *User) fetchHydrate(query string, args ...interface{}) (err error) {
	if u == nil {
		u = NewUser()
	}
	if err = db.SelectStruct(query, u, args...); err != nil {
		logger.LogIfDebug(err)
		return
	}
	// Assume the password stored in the DB is hashed.
	u.passwordHashed = true
	return
}

// Create saves the user to the DB.
func (u *User) Create() (err error) {
	if u.UUID != "" {
		err = validationError{fmt.Errorf("cannot recreate existing user")}
		return
	} else if err = ValidateEmail(u.Email); err != nil {
		return
	} else if u.Password == "" {
		err = validationError{fmt.Errorf("password cannot be empty")}
		return
	}

	if exists, xerr := u.Exists(); xerr != nil {
		err = xerr
		return
	} else if exists {
		err = validationError{fmt.Errorf("email is already registered")}
		return
	}

	id := uuid.New()
	u.UUID = uuid.Must(id, nil).String()
	u.Password = Hash(u.Password)
	u.CreatedAt = time.Now()

	err = db.Query(`
		INSERT INTO users (
			uuid, email, password, pw_func, pw_alg, pw_cost, pw_key_size,
			pw_nonce, pw_salt, created_at, updated_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		u.UUID, u.Email, u.Password, u.PwFunc, u.PwAlg, u.PwCost, u.PwKeySize,
		u.PwNonce, u.PwSalt, u.CreatedAt, u.UpdatedAt,
	)

	if err != nil {
		logger.LogIfDebug(err)
		return
	}
	u.passwordHashed = true
	return err
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
	u.passwordHashed = true
	return nil
}

// Exists checks if the user exists in the DB.
func (u *User) Exists() (bool, error) {
	if err := ValidateEmail(u.Email); err != nil {
		// swallow this error, it doesn't answer the question asked by this method.
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
	u.passwordHashed = false
	return u
}

// PwHashState can tell you whether or not the User's Password has been hashed.
func (u *User) PwHashState() PwHash {
	return PwHash{Value: u.Password, Hashed: u.passwordHashed}
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
