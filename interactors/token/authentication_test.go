package token_test

import (
	"testing"

	"github.com/dgrijalva/jwt-go"
	"github.com/rafaelespinoza/standardfile/db"
	"github.com/rafaelespinoza/standardfile/interactors/token"
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
		knownUser.Email = "foo@example.com"
		knownUser.Password = "testpassword123"
		knownUser.PwNonce = "stub_password_nonce"
		if err = knownUser.Create(); err != nil {
			t.Fatal(err)
		}
		if tok, err = models.EncodeToken(*knownUser); err != nil {
			t.Fatal(err)
		}
		if authenticatedUser, err = token.AuthenticateUser("Bearer " + tok); err != nil {
			t.Errorf("did not expect error; got %v", err)
		} else if authenticatedUser.UUID != knownUser.UUID {
			t.Errorf("users not equal\n%#v\n%#v\n", *authenticatedUser, *knownUser)
		}
	})

	t.Run("errors", func(t *testing.T) {
		t.Run("invalid header", func(t *testing.T) {
			if user, err := token.AuthenticateUser(""); err != token.ErrInvalidAuthHeader {
				t.Errorf(
					"expected error: %v; got %v",
					token.ErrInvalidAuthHeader, err,
				)
			} else if user != nil {
				t.Error("user should be nil")
			}

			if user, err := token.AuthenticateUser("foobar"); err != token.ErrInvalidAuthHeader {
				t.Errorf(
					"expected error: %v; got %v",
					token.ErrInvalidAuthHeader, err,
				)
			} else if user != nil {
				t.Error("user should be nil")
			}

			if user, err := token.AuthenticateUser("foo bar"); err != token.ErrInvalidAuthHeader {
				t.Errorf(
					"expected error: %v; got %v",
					token.ErrInvalidAuthHeader, err,
				)
			} else if user != nil {
				t.Error("user should be nil")
			}
		})

		t.Run("token validation", func(t *testing.T) {
			var user *models.User
			var err error
			user, err = token.AuthenticateUser("Bearer foobar")

			switch jerr := err.(type) {
			case *jwt.ValidationError:
				break // ok
			default:
				t.Errorf("expected %T, got %v", &jwt.ValidationError{}, jerr)
			}
			if user != nil {
				t.Error("user should be nil")
			}
		})

		t.Run("unknown user", func(t *testing.T) {
			var tok string
			var err error
			unknownUser := models.User{ // do not save in DB
				UUID:     "not a real UUID", // need this to attempt db lookup
				Email:    "foo@example.com",
				Password: "testpassword123",
				PwNonce:  "stub_password_nonce",
			}

			if tok, err = models.EncodeToken(unknownUser); err != nil {
				t.Fatal(err)
			}
			if user, err := token.AuthenticateUser("Bearer " + tok); err != token.ErrUnknownUser {
				t.Errorf(
					"expected error: %v; got %v",
					token.ErrUnknownUser, err,
				)
			} else if user != nil {
				t.Error("user should be nil")
			}
		})

		t.Run("invalid password", func(t *testing.T) {
			var tok string
			var err error
			knownUser := models.User{
				Email:    "foo@example.com",
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
			if err = knownUser.UpdatePassword(
				models.NewPassword{
					User:            knownUser,
					CurrentPassword: knownUser.Password,
					NewPassword:     knownUser.Password[1:],
				},
			); err != nil {
				t.Fatal(err)
			}
			if user, err := token.AuthenticateUser("Bearer " + tok); err != token.ErrPasswordInvalid {
				t.Errorf(
					"expected error: %v; got %v",
					token.ErrPasswordInvalid, err,
				)
			} else if user != nil {
				t.Error("user should be nil")
			}
		})
	})
}
