package interactors_test

import (
	"strings"
	"testing"

	"github.com/rafaelespinoza/standardfile/db"
	"github.com/rafaelespinoza/standardfile/errs"
	userInteractors "github.com/rafaelespinoza/standardfile/interactors/user"
	"github.com/rafaelespinoza/standardfile/models"
)

func init() {
	db.Init(":memory:")
}

func TestMakeAuthParams(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		user := models.NewUser()
		user.Email = t.Name() + "@example.com"
		user.Password = "testpassword123"
		user.PwNonce = "stub_password_nonce"
		var err error
		if err = user.Create(); err != nil {
			t.Fatal(err)
		}
		var params models.PwGenParams
		if params, err = userInteractors.MakeAuthParams(user.Email); err != nil {
			t.Error(err)
		}
		if params.PwFunc != "pbkdf2" {
			t.Errorf("wrong PwFunc; got %q, expected %q", params.PwFunc, "pbkdf2")
		}
		if params.PwAlg != "sha512" {
			t.Errorf("wrong PwAlg; got %q, expected %q", params.PwAlg, "sha512")
		}
		if params.PwCost < 100000 {
			t.Errorf("PwCost too cheap; got %d, expected >= %d", params.PwCost, 100000)
		}
		if params.PwNonce != "stub_password_nonce" {
			t.Errorf(
				"wrong value for PwNonce; got %q, expected %q",
				params.PwNonce, "stub_password_nonce",
			)
		}
	})

	t.Run("errors", func(t *testing.T) {
		user := models.NewUser()
		user.Email = t.Name() + "@example.com"
		user.Password = "testpassword123"
		var err error
		if err = user.Create(); err != nil {
			t.Fatal(err)
		}
		if _, err = userInteractors.MakeAuthParams(""); !errs.ValidationError(err) {
			t.Errorf("expected validation error but got %#v", err)
		}
		longEmail := strings.Repeat("foobar", 42) + "@example.com"
		if _, err = userInteractors.MakeAuthParams(longEmail); !errs.ValidationError(err) {
			t.Errorf("expected validation error but got %#v", err)
		}
		if _, err = userInteractors.MakeAuthParams("foobar"); !errs.ValidationError(err) {
			t.Errorf("expected validation error but got %#v", err)
		}
	})
}

func TestRegisterUser(t *testing.T) {
	user, tokenAfterRegistration, err := userInteractors.RegisterUser(
		userInteractors.RegisterUserParams{
			Email:    "user2@local",
			Password: "3cb5561daa49bd5b4438ad214a6f9a6d9b056a2c0b9a91991420ad9d658b8fac",
		},
	)
	if err != nil {
		t.Error(err)
	}
	if tokenAfterRegistration == "" {
		t.Error("token should not be empty")
	}
	if user == nil {
		t.Error("user should not be nil")
	}

	password := user.PwHashState()
	user, tokenAfterLogin, err := userInteractors.LoginUser(
		user.Email,
		&password,
	)
	if err != nil {
		t.Error(err)
	}
	if tokenAfterLogin == "" {
		t.Error("token should not be empty")
	}
	if tokenAfterLogin != tokenAfterRegistration {
		if tokenAfterLogin != tokenAfterRegistration {
			t.Errorf(
				"tokens should be the same\nafter registration: %q\nafter logging in:   %q",
				tokenAfterRegistration, tokenAfterLogin,
			)
		}
	}
}

func TestLoginUser(t *testing.T) {
	const plaintextPassword = "testpassword123"

	t.Run("ok", func(t *testing.T) {
		email := t.Name() + "@example.com"
		user := models.NewUser()
		user.Email = email
		user.Password = plaintextPassword
		user.PwNonce = "stub_password_nonce"
		if err := user.Create(); err != nil {
			t.Fatal(err)
		}

		password := user.PwHashState()
		user, token, err := userInteractors.LoginUser(email, &password)
		if err != nil {
			t.Error(err)
		}
		if token == "" {
			t.Error("token empty")
		}
		if user == nil {
			t.Error("user should not be nil")
		}
	})

	t.Run("errors", func(t *testing.T) {
		t.Run("wrong password", func(t *testing.T) {
			email := t.Name() + "@example.com"
			user := models.NewUser()
			user.Email = email
			user.Password = plaintextPassword
			user.PwNonce = "stub_password_nonce"
			if err := user.Create(); err != nil {
				t.Fatal(err)
			}

			user, token, err := userInteractors.LoginUser(email, &models.PwHash{Value: plaintextPassword[1:]})
			if err == nil {
				t.Errorf("expected error; got %v", err)
			}
			if token != "" {
				t.Error("token should be empty")
			}
			if user != nil {
				t.Error("user should be nil")
			}
		})

		t.Run("unregistered user", func(t *testing.T) {
			email := t.Name() + "@example.com"
			user := models.NewUser()
			user.Email = email
			user.Password = plaintextPassword
			user.PwNonce = "stub_password_nonce"

			password := user.PwHashState()
			user, token, err := userInteractors.LoginUser(email, &password)
			if err == nil {
				t.Errorf("expected error; got %v", err)
			}
			if token != "" {
				t.Error("token should be empty")
			}
			if user != nil {
				t.Error("user should be nil")
			}
		})
	})
}

func TestChangeUserPassword(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		var err error

		user, oldToken, err := userInteractors.RegisterUser(
			userInteractors.RegisterUserParams{
				Email:    t.Name() + "@example.com",
				Password: "testpassword123",
				PwNonce:  "stub_password_nonce",
			},
		)
		if err != nil {
			t.Fatalf("did not expect error; got %v", err)
		}
		newPassword := models.NewPassword{
			User:            *user,
			CurrentPassword: user.PwHashState(),
			NewPassword:     models.PwHash{Value: user.Password[1:]},
		}

		newToken, err := userInteractors.ChangeUserPassword(user, newPassword)
		if err != nil {
			t.Errorf("did not expect error; got %v", err)
		}
		if newToken == "" {
			t.Error("expected an output token, got empty")
		}
		if newToken == oldToken {
			t.Error("new token should not equal old token")
		}
	})

	t.Run("errors", func(t *testing.T) {
		t.Run("current password not provided", func(t *testing.T) {
			user := models.NewUser()
			user.Email = t.Name() + "@example.com"
			user.Password = "testpassword123"
			user.PwNonce = "stub_password_nonce"
			if err := user.Create(); err != nil {
				t.Fatalf("could not set up user; got %v", err)
			}

			newPassword := models.NewPassword{User: *user}
			token, err := userInteractors.ChangeUserPassword(user, newPassword)
			if !testError(t, err, errExpectations{
				messageFragment: "password",
				validation:      true,
			}) {
				t.Errorf("expected validation error, got %v", err)
			}
			if token != "" {
				t.Errorf("expected empty token, got %q", token)
			}
		})

		t.Run("no password nonce", func(t *testing.T) {
			user := models.NewUser()
			user.Email = t.Name() + "@example.com"
			user.Password = "testpassword123"
			if err := user.Create(); err != nil {
				t.Fatalf("could not set up user; got %v", err)
			}

			newPassword := models.NewPassword{
				User:            *user,
				CurrentPassword: user.PwHashState(),
			}
			token, err := userInteractors.ChangeUserPassword(user, newPassword)
			if !testError(t, err, errExpectations{
				messageFragment: "param",
				validation:      true,
			}) {
				t.Errorf("expected validation error, got %v", err)
			}
			if token != "" {
				t.Errorf("expected empty token, got %q", token)
			}
		})

		t.Run("current password incorrect", func(t *testing.T) {
			user := models.NewUser()
			user.Email = t.Name() + "@example.com"
			user.Password = "testpassword123"
			user.PwNonce = "stub_password_nonce"
			if err := user.Create(); err != nil {
				t.Fatalf("could not set up user; got %v", err)
			}

			currPW := user.PwHashState()
			currPW.Value = user.Password[1:]
			newPassword := models.NewPassword{
				User:            *user,
				CurrentPassword: currPW,
			}
			token, err := userInteractors.ChangeUserPassword(user, newPassword)
			if !testError(t, err, errExpectations{
				messageFragment: "password",
				validation:      true,
			}) {
				t.Errorf("expected validation error, got %v", err)
			}
			if token != "" {
				t.Errorf("expected empty token, got %q", token)
			}
		})
	})
}

type errExpectations struct {
	messageFragment string
	notFound        bool
	validation      bool
}

func testError(t *testing.T, actualErr error, expErr errExpectations) (ok bool) {
	ok = true
	t.Helper()

	expEmptyErr := expErr.messageFragment == "" && !expErr.notFound && !expErr.validation
	if actualErr != nil && expEmptyErr {
		t.Error("actual error is not nil, expected error is empty")
		ok = false
		return
	} else if actualErr == nil && !expEmptyErr {
		t.Error("actual error is nil, expected error is not empty")
		ok = false
		return
	}

	actNotFound := errs.NotFoundError(actualErr)
	if actNotFound != expErr.notFound {
		t.Errorf(
			"wrong values for NotFoundError; got %t, expected %t",
			actNotFound, expErr.notFound,
		)
		ok = false
	}

	actValidation := errs.ValidationError(actualErr)
	if actValidation != expErr.validation {
		t.Errorf(
			"wrong values for ValidationError; got %t, expected %t",
			actValidation, expErr.validation,
		)
		ok = false
	}

	actMessage := actualErr.Error()
	if !strings.Contains(actMessage, expErr.messageFragment) {
		t.Errorf("error message %q does not contain %q", actMessage, expErr.messageFragment)
		ok = false
	}
	return
}

func TestAuthenticateUser(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		var tok string
		var err error
		var knownUser, authenticatedUser *models.User
		knownUser = models.NewUser()
		knownUser.Email = t.Name() + "@example.com"
		knownUser.Password = "testpassword123"
		knownUser.PwNonce = "stub_password_nonce"
		if err = knownUser.Create(); err != nil {
			t.Fatal(err)
		}
		if tok, err = models.EncodeToken(*knownUser); err != nil {
			t.Fatal(err)
		}
		if authenticatedUser, err = userInteractors.AuthenticateUser("Bearer " + tok); err != nil {
			t.Errorf("did not expect error; got %v", err)
		} else if authenticatedUser.UUID != knownUser.UUID {
			t.Errorf("users not equal\n%#v\n%#v\n", *authenticatedUser, *knownUser)
		}
	})

	t.Run("errors", func(t *testing.T) {
		t.Run("invalid header", func(t *testing.T) {
			var user *models.User
			var err error
			expError := errExpectations{messageFragment: "header", notFound: false, validation: true}

			user, err = userInteractors.AuthenticateUser("")
			testError(t, err, expError)
			if user != nil {
				t.Error("user should be nil")
			}

			user, err = userInteractors.AuthenticateUser("foobar")
			testError(t, err, expError)
			if user != nil {
				t.Error("user should be nil")
			}

			user, err = userInteractors.AuthenticateUser("foo bar")
			testError(t, err, expError)
			if user != nil {
				t.Error("user should be nil")
			}
		})

		t.Run("token validation", func(t *testing.T) {
			expError := errExpectations{messageFragment: "token", notFound: false, validation: true}
			user, err := userInteractors.AuthenticateUser("Bearer foobar")

			testError(t, err, expError)
			if user != nil {
				t.Error("user should be nil")
			}
		})

		t.Run("unknown user", func(t *testing.T) {
			var tok string
			var err error
			unknownUser := models.User{ // do not save in DB
				UUID:     "not a real UUID", // need this to attempt db lookup
				Email:    t.Name() + "@example.com",
				Password: "testpassword123",
				PwNonce:  "stub_password_nonce",
			}

			if tok, err = models.EncodeToken(unknownUser); err != nil {
				t.Fatal(err)
			}

			user, err := userInteractors.AuthenticateUser("Bearer " + tok)
			testError(t, err, errExpectations{"email", true, false})
			if user != nil {
				t.Error("user should be nil")
			}
		})

		t.Run("invalid password", func(t *testing.T) {
			var tok string
			var err error
			knownUser := models.User{
				Email:    t.Name() + "@example.com",
				Password: "testpassword123",
				PwNonce:  "stub_password_nonce",
			}

			if err = knownUser.Create(); err != nil {
				t.Fatal(err)
			}
			if tok, err = models.EncodeToken(knownUser); err != nil {
				t.Fatal(err)
			}

			// make a legit token stale by updating password
			if _, err = userInteractors.ChangeUserPassword(
				&knownUser,
				models.NewPassword{
					User:            knownUser,
					CurrentPassword: knownUser.PwHashState(),
					NewPassword:     models.PwHash{Value: knownUser.Password[1:]},
				},
			); err != nil {
				t.Fatal(err)
			}
			expError := errExpectations{
				messageFragment: "password",
				notFound:        false,
				validation:      true,
			}
			user, err := userInteractors.AuthenticateUser("Bearer " + tok)
			testError(t, err, expError)
			if user != nil {
				t.Error("user should be nil")
			}
		})
	})
}
