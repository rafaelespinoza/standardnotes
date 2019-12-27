package models

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/rafaelespinoza/standardfile/encryption"
	"github.com/rafaelespinoza/standardfile/logger"
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

// TimeToToken generates a token for the time.
func TimeToToken(date time.Time) string {
	return base64.URLEncoding.EncodeToString(
		[]byte(
			fmt.Sprintf(
				"1:%d", // TODO: make use of "version" 1 and 2. (part before :)
				date.UnixNano(),
			),
		),
	)
}

// TokenToTime converts a token to a time.
func TokenToTime(token string) time.Time {
	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		logger.LogIfDebug(err)
		return time.Now()
	}
	parts := strings.Split(string(decoded), ":")
	str, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		logger.LogIfDebug(err)
		return time.Now()
	}
	// TODO: output "version" 1, 2 differently. See
	// `lib/sync_engine/abstract/sync_manager.rb` in the ruby sync-server
	return time.Time(time.Unix(0, int64(str)))
}
