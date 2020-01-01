package interactors_test

import (
	"testing"

	"github.com/rafaelespinoza/standardfile/db"
	"github.com/rafaelespinoza/standardfile/interactors"
	"github.com/rafaelespinoza/standardfile/models"
)

func init() {
	db.Init(":memory:")
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
		if authenticatedUser, err = interactors.AuthenticateUser("Bearer " + tok); err != nil {
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

			user, err = interactors.AuthenticateUser("")
			testError(t, err, expError)
			if user != nil {
				t.Error("user should be nil")
			}

			user, err = interactors.AuthenticateUser("foobar")
			testError(t, err, expError)
			if user != nil {
				t.Error("user should be nil")
			}

			user, err = interactors.AuthenticateUser("foo bar")
			testError(t, err, expError)
			if user != nil {
				t.Error("user should be nil")
			}
		})

		t.Run("token validation", func(t *testing.T) {
			expError := errExpectations{messageFragment: "token", notFound: false, validation: true}
			user, err := interactors.AuthenticateUser("Bearer foobar")

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

			user, err := interactors.AuthenticateUser("Bearer " + tok)
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
			if _, err = interactors.ChangeUserPassword(
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
			user, err := interactors.AuthenticateUser("Bearer " + tok)
			testError(t, err, expError)
			if user != nil {
				t.Error("user should be nil")
			}
		})
	})
}
