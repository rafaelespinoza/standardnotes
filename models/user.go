package models

import (
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/kisielk/sqlstruct"
	"github.com/rafaelespinoza/standardfile/db"
	"github.com/rafaelespinoza/standardfile/encryption"
	"github.com/rafaelespinoza/standardfile/logger"
	uuid "github.com/satori/go.uuid"
)

// User is the user type
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

var _ db.MigratingUser = (*User)(nil)

func (u *User) GetEmail() string   { return u.Email }
func (u *User) GetPwNonce() string { return u.PwNonce }
func (u *User) GetUUID() string    { return u.UUID }

// Params is params type
type Params struct {
	PwFunc    string `json:"pw_func"     sql:"pw_func"`
	PwAlg     string `json:"pw_alg"      sql:"pw_alg"`
	PwCost    int    `json:"pw_cost"     sql:"pw_cost"`
	PwKeySize int    `json:"pw_key_size" sql:"pw_key_size"`
	PwSalt    string `json:"pw_salt"     sql:"pw_salt"`
	Version   string `json:"version"`
}

// NewPassword - incomming json password change
type NewPassword struct {
	User
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// UserClaims - jwt claims
type UserClaims struct {
	UUID   string `json:"uuid"`
	PwHash string `json:"pw_hash"`
	jwt.StandardClaims
}

//NewUser - user constructor
func NewUser() User {
	user := User{}
	user.PwCost = 100000
	user.PwAlg = "sha512"
	user.PwKeySize = 512
	user.PwFunc = "pbkdf2"
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	return user
}

//LoadValue - hydrate struct from map
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

//save - save current user into DB
func (u *User) create() error {
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

	err := db.Query("INSERT INTO users (uuid, email, password, pw_func, pw_alg, pw_cost, pw_key_size, pw_nonce, pw_auth, pw_salt, created_at, updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", u.UUID, u.Email, u.Password, u.PwFunc, u.PwAlg, u.PwCost, u.PwKeySize, u.PwNonce, u.PwAuth, u.PwSalt, u.CreatedAt, u.UpdatedAt)

	if err != nil {
		logger.Log(err)
	}

	return err
}

//UpdatePassword - update password
func (u *User) UpdatePassword(np NewPassword) error {
	if u.UUID == "" {
		return fmt.Errorf("Unknown user")
	}

	u.Password = Hash(np.NewPassword)
	u.PwCost = np.PwCost
	u.PwSalt = np.PwSalt
	u.PwNonce = np.PwNonce

	u.UpdatedAt = time.Now()
	// TODO: validate incomming pw params
	err := db.Query("UPDATE `users` SET `password`=?, `pw_cost`=?, `pw_salt`=?, `pw_nonce`=?, `updated_at`=? WHERE `uuid`=?", u.Password, u.PwCost, u.PwSalt, u.PwNonce, u.UpdatedAt, u.UUID)

	if err != nil {
		logger.Log(err)
		return err
	}

	return nil
}

//UpdateParams - update params
func (u *User) UpdateParams(p Params) error {
	if u.UUID == "" {
		return fmt.Errorf("Unknown user")
	}

	u.UpdatedAt = time.Now()
	err := db.Query("UPDATE `users` SET `pw_func`=?, `pw_alg`=?, `pw_cost`=?, `pw_key_size`=?, `pw_salt`=?, `updated_at`=? WHERE `uuid`=?", u.PwFunc, u.PwAlg, u.PwCost, u.PwKeySize, u.PwSalt, time.Now(), u.UUID)

	if err != nil {
		logger.Log(err)
		return err
	}

	return nil
}

//Register - creates user and returns token
func (u *User) Register() (string, error) {
	err := u.create()
	if err != nil {
		return "", err
	}

	token, err := u.Login(u.Email, u.Password)
	if err != nil {
		return "", fmt.Errorf("Registration failed")
	}

	return token, nil
}

//Exists - checks if current user exists in DB
func (u User) Exists() bool {
	uuid, err := db.SelectFirst("SELECT `uuid` FROM `users` WHERE `email`=?", u.Email)

	if err != nil {
		logger.Log(err)
		return false
	}

	return uuid != ""
}

//Login - logins user
func (u *User) Login(email, password string) (string, error) {
	u.loadByEmailAndPassword(email, Hash(password))

	if u.UUID == "" {
		return "", fmt.Errorf("Invalid email or password")
	}

	token, err := u.CreateToken()
	if err != nil {
		return "", err
	}

	return token, nil
}

//LoadByUUID - loads user info from DB
func (u *User) LoadByUUID(uuid string) bool {
	_, err := db.SelectStruct(fmt.Sprintf("SELECT %s FROM `users` WHERE `uuid`=?", sqlstruct.Columns(User{})), u, uuid)
	if err != nil {
		logger.Log("Load err:", err)
		return false
	}

	return true
}

//CreateToken - will create JWT token
func (u User) CreateToken() (string, error) {
	claims := UserClaims{
		u.UUID,
		u.Password,
		jwt.StandardClaims{
			IssuedAt: time.Now().Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(encryption.SigningKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (u *User) loadByEmail(email string) {
	_, err := db.SelectStruct("SELECT * FROM `users` WHERE `email`=?", u, email)
	if err != nil {
		logger.Log(err)
	}
}

func (u *User) loadByEmailAndPassword(email, password string) {
	_, err := db.SelectStruct("SELECT * FROM `users` WHERE `email`=? AND `password`=?", u, email, password)
	if err != nil {
		logger.Log(err)
	}
}

// GetParams returns auth parameters by email
func (u User) GetParams(email string) map[string]interface{} {
	u.loadByEmail(email)
	params := map[string]interface{}{}

	if u.Email == "" {
		return params
	}

	params["version"] = "003"
	params["pw_cost"] = u.PwCost
	params["identifier"] = u.Email

	if u.PwFunc != "" {
		params["pw_func"] = u.PwFunc
		params["pw_alg"] = u.PwAlg
		params["pw_key_size"] = u.PwKeySize
	}

	if u.PwNonce != "" {
		params["pw_nonce"] = u.PwNonce
	}

	if u.PwSalt == "" {
		nonce := u.PwNonce
		if nonce == "" {
			nonce = "a04a8fe6bcb19ba61c5c0873d391e987982fbbd4"
		}
		u.PwSalt = encryption.Salt(u.Email, nonce)
	}
	params["pw_salt"] = u.PwSalt

	return params
}

//Validate - validates password from jwt
func (u User) Validate(password string) bool {
	return password == u.Password
}

//ToJSON - return map without pw and nonce
func (u User) ToJSON() interface{} {
	u.Password = ""
	u.PwNonce = ""
	return u
}

//Hash - sha256 hash function
func Hash(input string) string {
	return strings.Replace(fmt.Sprintf("% x", sha256.Sum256([]byte(input))), " ", "", -1)
}
