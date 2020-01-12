package models

import (
	"fmt"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
)

var _SigningKey = []byte{}

func init() {
	key := os.Getenv("SECRET_KEY_BASE")
	if key == "" {
		key = "qA6irmDikU6RkCM4V0cJiUJEROuCsqTa1esexI4aWedSv405v8lw4g1KB1nQVsSdCrcyRlKFdws4XPlsArWwv9y5Xr5Jtkb11w1NxKZabOUa7mxjeENuCs31Y1Ce49XH9kGMPe0ms7iV7e9F6WgnsPFGOlIA3CwfGyr12okas2EsDd71SbSnA0zJYjyxeCVCZJWISmLB"
	}
	_SigningKey = []byte(key)
}

// A Token provides user authentication using a JWT.
type Token interface {
	Claims() Claims
	Valid() bool
}

type webToken struct {
	token  *jwt.Token
	claims Claims
}

var _ Token = (*webToken)(nil)

func (t *webToken) Claims() Claims { return t.claims }
func (t *webToken) Valid() bool    { return t.token.Valid }

// Claims is the JWT payload.
type Claims interface {
	// UUID should return the UUID of the User represented in the token.
	UUID() string
	// Hash should return the hashed password of the User.
	Hash() string
	// Valid should return an error to signal an invalid token, or otherwise
	// return nil.
	Valid() error
}

// userClaims is a set of JWT claims that implements the Claims interface.
type userClaims struct {
	PwHash string
	UserID string
	jwt.StandardClaims
}

var _ Claims = (*userClaims)(nil)

func (c *userClaims) Hash() string { return c.PwHash }
func (c *userClaims) UUID() string { return c.UserID }

// EncodeToken makes a JWT token for a User.
func EncodeToken(u User) (string, error) {
	claims := userClaims{
		UserID: u.UUID,
		PwHash: u.Password,
		StandardClaims: jwt.StandardClaims{
			IssuedAt: time.Now().Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(_SigningKey)
	if err != nil {
		return "", err
	}

	return signedToken, nil
}

// DecodeToken wraps the jwt library's token parsing function.
func DecodeToken(encodedToken string) (tok Token, err error) {
	claims := new(userClaims)
	out, err := new(jwt.Parser).ParseWithClaims(
		encodedToken,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf(
					"unexpected signing method: %v",
					token.Header["alg"],
				)
			}
			return _SigningKey, nil
		},
	)
	if err != nil {
		return
	}
	tok = &webToken{token: out, claims: claims}
	return
}
