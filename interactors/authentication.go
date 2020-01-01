package interactors

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rafaelespinoza/standardfile/logger"
	"github.com/rafaelespinoza/standardfile/models"
)

type authenticationError struct {
	error
	notFound   bool
	validation bool
}

func (e authenticationError) NotFound() bool   { return e.notFound }
func (e authenticationError) Validation() bool { return e.validation }

func AuthenticateUser(header string) (user *models.User, err error) {
	authHeaderParts := strings.Split(header, " ")

	if len(authHeaderParts) != 2 || strings.ToLower(authHeaderParts[0]) != "bearer" {
		err = authenticationError{
			error:      errors.New("invalid authorization header"),
			validation: true,
		}
		return
	}

	var token models.Token
	if token, err = models.DecodeToken(authHeaderParts[1]); err != nil {
		err = authenticationError{error: err, validation: true}
		return
	}

	if !token.Valid() {
		err = authenticationError{
			error:      fmt.Errorf("invalid token"),
			validation: true,
		}
		return
	}
	claims := token.Claims()
	logger.LogIfDebug("token is valid, claims: ", claims)

	if user, err = models.LoadUserByUUID(claims.UUID()); err != nil {
		logger.LogIfDebug("LoadUserByUUID error: ", err)
		err = maybeMutateError(err)
		return
	}

	if !user.Validate(claims.Hash()) {
		user = nil
		err = authenticationError{
			error:      errors.New("password does not match"),
			validation: true,
		}
	}
	return
}
