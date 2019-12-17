package interactors

import (
	"errors"
	"regexp"

	"github.com/rafaelespinoza/standardfile/models"
)

var (
	ErrInvalidEmail = errors.New("email invalid")
)

func MakeAuthParams(email string) (params models.Params, err error) {
	if err = validateEmail(email); err != nil {
		return
	}
	user := models.NewUser()
	user.LoadByEmail(email)
	params = models.MakeAuthParams(*user)
	return
}

func validateEmail(email string) error {
	if len(email) < 1 || len(email) > 255 {
		return ErrInvalidEmail
	}
	match, err := regexp.MatchString(".+@.+", email)
	if err != nil {
		return err
	}
	if !match {
		return ErrInvalidEmail
	}
	return nil
}
