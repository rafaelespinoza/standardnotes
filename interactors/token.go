package interactors

import (
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/rafaelespinoza/standardfile/encryption"
	"github.com/rafaelespinoza/standardfile/models"
)

// CreateUserToken makes a JWT token.
func CreateUserToken(u models.User) (string, error) {
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
