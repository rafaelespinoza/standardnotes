package token

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rafaelespinoza/standardfile/logger"
	"github.com/rafaelespinoza/standardfile/models"
)

var (
	ErrInvalidAuthHeader = errors.New("invalid authorization header")
	ErrUnknownUser       = errors.New("unknown user")
	ErrPasswordInvalid   = errors.New("password does not match")
)

func AuthenticateUser(header string) (user *models.User, err error) {
	authHeaderParts := strings.Split(header, " ")

	if len(authHeaderParts) != 2 || strings.ToLower(authHeaderParts[0]) != "bearer" {
		err = ErrInvalidAuthHeader
		return
	}

	var token models.Token
	if token, err = models.DecodeToken(authHeaderParts[1]); err != nil {
		return
	}

	if !token.Valid() {
		err = fmt.Errorf("invalid token")
		return
	}
	claims := token.Claims()
	logger.LogIfDebug("token is valid, claims: ", claims)

	if user, err = models.LoadUserByUUID(claims.UUID()); err != nil {
		logger.LogIfDebug("LoadUserByUUID error: ", err)
		err = ErrUnknownUser
		return
	}

	if !user.Validate(claims.Hash()) {
		user = nil
		err = ErrPasswordInvalid
	}
	return
}
