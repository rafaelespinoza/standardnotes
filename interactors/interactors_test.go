package interactors_test

import (
	"strings"
	"testing"

	"github.com/rafaelespinoza/standardfile/db"
	"github.com/rafaelespinoza/standardfile/interactors"
	"github.com/rafaelespinoza/standardfile/models"
)

func init() {
	db.Init(":memory:")
}

func TestMakeAuthParams(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		user := models.NewUser()
		user.Email = "foo@example.com"
		user.Password = "testpassword123"
		user.PwNonce = "stub_password_nonce"
		var err error
		if err = user.Create(); err != nil {
			t.Fatal(err)
		}
		var params models.Params
		if params, err = interactors.MakeAuthParams(user.Email); err != nil {
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
	t.Run("invalid email", func(t *testing.T) {
		user := models.NewUser()
		user.Email = "foo@example.com"
		user.Password = "testpassword123"
		var err error
		if err = user.Create(); err != nil {
			t.Fatal(err)
		}
		if _, err = interactors.MakeAuthParams(""); err != interactors.ErrInvalidEmail {
			t.Errorf("expected %v but got %v", interactors.ErrInvalidEmail, err)
		}
		longEmail := strings.Repeat("foobar", 42) + "@example.com"
		if _, err = interactors.MakeAuthParams(longEmail); err != interactors.ErrInvalidEmail {
			t.Errorf("expected %v but got %v", interactors.ErrInvalidEmail, err)
		}
		if _, err = interactors.MakeAuthParams("foobar"); err != interactors.ErrInvalidEmail {
			t.Errorf("expected %v but got %v", interactors.ErrInvalidEmail, err)
		}
	})
}

func TestRegisterLoginUser(t *testing.T) {
	user := models.User{
		Email:     "user2@local",
		Password:  "3cb5561daa49bd5b4438ad214a6f9a6d9b056a2c0b9a91991420ad9d658b8fac",
		PwCost:    101000,
		PwSalt:    "685bdeca99977eb0a30a68284d86bbb322c3b0ee832ffe4b6b76bd10fe7b8362",
		PwAlg:     "sha512",
		PwKeySize: 512,
		PwFunc:    "pbkdf2",
	}
	tokenAfterRegistration, err := interactors.RegisterUser(&user)
	if err != nil {
		t.Error("Register failed", err)
		return
	}
	if len(tokenAfterRegistration) < 1 {
		t.Error("token empty")
		return
	}

	tokenAfterLogin, err := interactors.LoginUser(user, user.Email, user.Password)
	if err != nil {
		t.Error("Login failed", err)
		return
	}
	if len(tokenAfterLogin) < 1 {
		t.Error("token empty")
	}
	if tokenAfterLogin != tokenAfterRegistration {
		t.Errorf(
			"tokens should be the same\nafter registration: %q\nafter logging in:   %q",
			tokenAfterRegistration, tokenAfterLogin,
		)
	}
}
