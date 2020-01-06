package models_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/rafaelespinoza/standardnotes/models"
)

func TestNewPassword(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		const plaintextPassword = "testpassword123"

		t.Run("unmarshal", func(t *testing.T) {
			var _ json.Unmarshaler = (*models.NewPassword)(nil)
			var input bytes.Buffer
			if _, err := fmt.Fprintf(
				&input,
				`{"user": {}, "current_password": "%s", "new_password": "%s"}`,
				plaintextPassword, plaintextPassword[1:],
			); err != nil {
				t.Fatal(err)
			}
			expected := models.NewPassword{
				User:            models.User{},
				CurrentPassword: models.PwHash{Value: plaintextPassword},
				NewPassword:     models.PwHash{Value: plaintextPassword[1:]},
			}
			var actual models.NewPassword
			if err := json.Unmarshal(input.Bytes(), &actual); err != nil {
				t.Error(err)
			}
			if actual.CurrentPassword != expected.CurrentPassword {
				t.Errorf(
					"wrong CurrentPassword\ngot: %#v\nexp: %#v\n",
					actual.CurrentPassword, expected.CurrentPassword,
				)
			}
			if actual.NewPassword != expected.NewPassword {
				t.Errorf(
					"wrong NewPassword\ngot: %#v\nexp: %#v\n",
					actual.NewPassword, expected.NewPassword,
				)
			}
		})

		t.Run("marshal", func(t *testing.T) {
			var _ json.Marshaler = (*models.NewPassword)(nil)
			var expected bytes.Buffer
			if _, err := fmt.Fprintf(
				&expected,
				`{"uuid":"","email":"","pw_func":"","pw_alg":"","pw_cost":0,"pw_key_size":0,"created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z","current_password":"%s","new_password":"%s"}`,
				plaintextPassword, plaintextPassword[1:],
			); err != nil {
				t.Fatal(err)
			}
			out, err := json.Marshal(models.NewPassword{
				User:            models.User{},
				CurrentPassword: models.PwHash{Value: plaintextPassword},
				NewPassword:     models.PwHash{Value: plaintextPassword[1:]},
			})
			if err != nil {
				t.Error(err)
			}
			exp := expected.Bytes()
			if !bytes.Equal(out, exp) {
				t.Errorf(
					"actual did not equal expected\nactual:   %q\nexpected: %q",
					string(out), string(exp),
				)
			}
		})
	})
}
