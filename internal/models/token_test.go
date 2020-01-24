package models_test

import (
	"strings"
	"testing"

	"github.com/rafaelespinoza/standardnotes/internal/models"
)

func TestEncodeToken(t *testing.T) {
	user := models.NewUser()
	user.UUID = "just-a-stub-uuid"
	user.Password = models.Hash("testpassword123")

	token, err := models.EncodeToken(*user)
	if err != nil {
		t.Errorf("did not expect error; got %v", err)
	}
	if token == "" {
		t.Error("got empty token")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Errorf(
			"token should come in 3 parts, separated by a %q; got %d",
			".", len(parts),
		)
	}

	// no particular reason for this number other than it's not absurdly short.
	const longAndSecure = 32
	for i, part := range parts {
		if len(part) < longAndSecure {
			t.Errorf("length of part [%d], %d, is too short", len(part), i)
		}
	}
}

func TestDecodeToken(t *testing.T) {
	const userUUID = "just-a-stub-uuid"
	const plaintextPassword = "testpassword123"
	user := models.NewUser()
	user.UUID = userUUID
	user.Password = models.Hash(plaintextPassword)
	encodedToken, err := models.EncodeToken(*user)
	if err != nil {
		t.Fatal(err)
	} else if encodedToken == "" {
		t.Fatalf("need non-empty encoded token for tests")
	}

	t.Run("ok", func(t *testing.T) {
		decodedToken, err := models.DecodeToken(encodedToken)
		if err != nil {
			t.Errorf("did not expect error; got %v", err)
		} else if decodedToken == nil {
			t.Fatal("did not expect nil decoded token")
		}

		if claims := decodedToken.Claims(); claims == nil {
			t.Error("expected non-nil claims")
		} else if claims.Hash() != models.Hash(plaintextPassword) {
			t.Errorf(
				"expected Hash to be the hashed password\ngot %q\nexp %q",
				claims.Hash(), models.Hash(plaintextPassword),
			)
		} else if claims.UUID() != userUUID {
			t.Errorf(
				"expected UUID to be user's UUID\ngot %q\nexp %q",
				claims.UUID(), userUUID,
			)
		} else if err := claims.Valid(); err != nil {
			t.Errorf("expected Valid to output a valid token; got %v", err)
		}

		if valid := decodedToken.Valid(); !valid {
			t.Errorf("expected Valid to output a valid token; got %v", valid)
		}
	})

	t.Run("errors", func(t *testing.T) {
		decodedToken, err := models.DecodeToken(encodedToken[1:])
		if err == nil {
			t.Errorf("expected error; got %v", err)
		}
		if decodedToken != nil {
			t.Errorf("expected nil decoded token; got %v", decodedToken)
		}
	})
}
