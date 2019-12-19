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
