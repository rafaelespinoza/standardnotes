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

// CreateUserToken makes a JWT token.
func CreateUserToken(u User) (string, error) {
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

// UserClaims is a set of JWT claims.
type UserClaims struct {
	UUID   string `json:"uuid"` // TODO: verify that this is User.UUID
	PwHash string `json:"pw_hash"`
	jwt.StandardClaims
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
