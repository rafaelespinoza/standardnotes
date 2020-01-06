package models

import (
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/rafaelespinoza/standardnotes/encryption"
)

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
	signedToken, err := token.SignedString(encryption.SigningKey)
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
			return encryption.SigningKey, nil
		},
	)
	if err != nil {
		return
	}
	tok = &webToken{token: out, claims: claims}
	return
}
