package interactors

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/rafaelespinoza/standardfile/jobs"
	"github.com/rafaelespinoza/standardfile/models"
)

var (
	ErrInvalidEmail                   = errors.New("email invalid")
	ErrNoPasswordProvidedDuringChange = errors.New(strings.TrimSpace(`
		your current password is required to change your password.
		Please update your application if you do not see this option.`,
	))
	ErrPasswordIncorrect = errors.New(
		"the current password you entered is incorrect. Please try again",
	)
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

// LoginUser signs in the user. It returns a token on success, otherwise an error.
func LoginUser(u models.User, email, password string) (token string, err error) {
	u.LoadByEmailAndPassword(email, password)

	if u.UUID == "" {
		err = fmt.Errorf("invalid email or password")
		return
	}

	token, err = models.CreateUserToken(u)
	if err != nil {
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

// Register creates a new user and returns a token.
func RegisterUser(u *models.User) (token string, err error) {
	err = u.Create()
	if err != nil {
		return "", err
	}

	token, err = LoginUser(*u, u.Email, u.Password)
	if err != nil {
		err = fmt.Errorf("registration failed")
		return
	}

	params := jobs.RegistrationJobParams{
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	}
	if err = jobs.PerformRegistrationJob(params); err != nil {
		log.Printf("error performing job; %v\n", err)
	}
	return
}

func ChangeUserPassword(user *models.User, password models.NewPassword) (token string, err error) {
	if len(password.CurrentPassword) == 0 {
		err = ErrNoPasswordProvidedDuringChange
		return
	}

	// TODO: check password nonce

	if _, err = LoginUser(*user, password.Email, password.CurrentPassword); err != nil {
		if ierr := handleFailedAuthAttempt(*user); ierr != nil {
			err = ierr
		}
		err = ErrPasswordIncorrect
		return
	}
	if err = handleSuccessfulAuthAttempt(*user); err != nil {
		return
	}

	if err = user.UpdatePassword(password); err != nil {
		return
	}

	// now login again with new password
	if token, err = LoginUser(*user, user.Email, user.Password); err != nil {
		err = ErrPasswordIncorrect
		return
	}
	return
}
