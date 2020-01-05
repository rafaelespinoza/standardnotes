package interactors

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/rafaelespinoza/standardfile/errs"
	"github.com/rafaelespinoza/standardfile/jobs"
	"github.com/rafaelespinoza/standardfile/models"
)

var (
	errMissingNewAuthParams = errors.New(
		"the change password request is missing new auth parameters, please try again",
	)
	errNoPasswordProvidedDuringChange = errors.New(strings.TrimSpace(`
		your current password is required to change your password,
		please update your application if you do not see this option.`,
	))
	errPasswordIncorrect = errors.New(
		"the current password you entered is incorrect, please try again",
	)
	// errInvalidEmailOrPassword is a fallback error.
	errInvalidEmailOrPassword = errors.New("invalid email or password")
)

func MakeAuthParams(email string) (params models.PwGenParams, err error) {
	if err = models.ValidateEmail(email); err != nil {
		return
	}
	var user *models.User
	if user, err = models.LoadUserByEmail(email); err != nil {
		err = maybeMutateError(err)
		return
	}
	params = models.MakePwGenParams(*user)
	return
}

type RegisterUserParams struct {
	API        string
	Email      string
	Identifier string
	Password   string
	PwCost     int    `json:"pw_cost"`
	PwNonce    string `json:"pw_nonce"`
	Version    string
}

// Register creates a new user and returns a token.
func RegisterUser(params RegisterUserParams) (user *models.User, token string, err error) {
	user = models.NewUser()
	user.Email = params.Email
	user.Password = params.Password
	user.PwCost = params.PwCost
	user.PwNonce = params.PwNonce
	err = user.Create()
	if err != nil {
		user = nil
		return
	}

	password := user.PwHashState()
	user, token, err = LoginUser(user.Email, &password)
	if err != nil {
		user = nil
		err = fmt.Errorf("registration failed; %v", err)
		return
	}

	if err = jobs.PerformRegistrationJob(
		jobs.RegistrationJobParams{
			Email:     user.Email,
			CreatedAt: user.CreatedAt.UTC(),
		},
	); err != nil {
		// log it, but keep going
		log.Printf("error performing job; %v\n", err)
		err = nil
	}
	return
}

// LoginUser signs in the user. It returns a token on success, otherwise an error.
func LoginUser(email string, password *models.PwHash) (user *models.User, token string, err error) {
	password.Hash()
	if user, err = models.LoadUserByEmailAndPassword(email, password.Value); err != nil {
		err = maybeMutateError(err)
		return
	}

	if user.UUID == "" {
		user = nil
		err = authenticationError{
			error:      errInvalidEmailOrPassword,
			validation: true,
		}
		return
	}

	token, err = models.EncodeToken(*user)
	if err != nil {
		user = nil
		return
	}

	return
}

func handleFailedAuthAttempt(u models.User) error {
	// TODO increment number of failed login attempts for user (also store in
	// the db). If it 's past a limit, return an error and lockout user.
	return nil
}

func handleSuccessfulAuthAttempt(u models.User) error {
	// TODO reset number of failed attempts to 0.
	return nil
}

func ChangeUserPassword(user *models.User, password models.NewPassword) (token string, err error) {
	if len(password.CurrentPassword.Value) == 0 {
		err = authenticationError{error: errNoPasswordProvidedDuringChange, validation: true}
		return
	} else if len(password.PwNonce) == 0 {
		err = authenticationError{error: errMissingNewAuthParams, validation: true}
		return
	}

	if _, _, err = LoginUser(password.Email, &password.CurrentPassword); err != nil {
		if ierr := handleFailedAuthAttempt(*user); ierr != nil {
			err = ierr
		}
		err = authenticationError{error: errPasswordIncorrect, validation: true}
		return
	}
	if err = handleSuccessfulAuthAttempt(*user); err != nil {
		return
	}

	updates := user.MakeSaferCopy()
	password.NewPassword.Hash()
	updates.Password = password.NewPassword.Value
	updates.PwNonce = user.PwNonce
	if err = user.Update(updates); err != nil {
		return
	}

	// now login again with new password
	newPassword := models.PwHash{
		Value: user.Password,
		// Don't rehash, login would fail. It was already hashed before updating
		// user in DB.
		Hashed: true,
	}
	if user, token, err = LoginUser(user.Email, &newPassword); err != nil {
		err = authenticationError{error: errPasswordIncorrect, validation: true}
		return
	}
	return
}

var (
	_ errs.NotFound   = (*authenticationError)(nil)
	_ errs.Validation = (*authenticationError)(nil)
)

func maybeMutateError(in error) (out error) {
	if errs.NotFoundError(in) {
		out = authenticationError{
			error:    errInvalidEmailOrPassword,
			notFound: true,
		}
	} else if errs.ValidationError(in) {
		out = authenticationError{error: in, validation: true}
	} else {
		out = in
	}
	return
}
