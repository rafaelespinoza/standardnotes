package models_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/rafaelespinoza/standardnotes/internal/models"
)

func TestNewPassword(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		const (
			plaintextPassword = "testpassword123"
			stubPwNonce       = "1b1fba800047cb575942e9570db5b2bb0c8437b24bd7c0705f4b8cacde161f57"
			stubID            = "8327c5c5-fe4b-4ccb-b941-49ee668a82c0@example.com"
			version           = "003"
			api               = "20190520"
			pwCost            = 11000
		)

		t.Run("unmarshal", func(t *testing.T) {
			var _ json.Unmarshaler = (*models.PwChangeParams)(nil)
			var input bytes.Buffer
			if _, err := fmt.Fprintf(
				&input,
				`{"current_password":"%s","new_password":"%s","api":"%s","identifier":"%s","pw_cost":%d,"pw_nonce":"%s","version":"%s"}`,
				plaintextPassword, plaintextPassword[1:], api, stubID, pwCost, stubPwNonce, version,
			); err != nil {
				t.Fatal(err)
			}
			expected := models.PwChangeParams{
				CurrentPassword: models.PwHash{Value: plaintextPassword},
				NewPassword:     models.PwHash{Value: plaintextPassword[1:]},
				API:             api,
				Identifier:      stubID,
				PwCost:          pwCost,
				PwNonce:         stubPwNonce,
				Version:         version,
			}
			var actual models.PwChangeParams
			if err := json.Unmarshal(input.Bytes(), &actual); err != nil {
				t.Error(err)
			}
			if actual.API != expected.API {
				t.Errorf(
					"wrong API\ngot: %q\nexp: %q\n",
					actual.API, expected.API,
				)
			}
			if actual.Identifier != expected.Identifier {
				t.Errorf(
					"wrong Identifier\ngot: %q\nexp: %q\n",
					actual.Identifier, expected.Identifier,
				)
			}
			if actual.PwCost != expected.PwCost {
				t.Errorf(
					"wrong PwCost\ngot: %q\nexp: %q\n",
					actual.PwCost, expected.PwCost,
				)
			}
			if actual.PwNonce != expected.PwNonce {
				t.Errorf(
					"wrong PwNonce\ngot: %q\nexp: %q\n",
					actual.PwNonce, expected.PwNonce,
				)
			}
			if actual.Version != expected.Version {
				t.Errorf(
					"wrong Version\ngot: %q\nexp: %q\n",
					actual.Version, expected.Version,
				)
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
			var _ json.Marshaler = (*models.PwChangeParams)(nil)
			var expected bytes.Buffer
			if _, err := fmt.Fprintf(
				&expected,
				`{"current_password":"%s","new_password":"%s","api":"%s","identifier":"%s","pw_cost":%d,"pw_nonce":"%s","version":"%s"}`,
				plaintextPassword, plaintextPassword[1:], api, stubID, pwCost, stubPwNonce, version,
			); err != nil {
				t.Fatal(err)
			}
			out, err := json.Marshal(models.PwChangeParams{
				CurrentPassword: models.PwHash{Value: plaintextPassword},
				NewPassword:     models.PwHash{Value: plaintextPassword[1:]},
				API:             api,
				Identifier:      stubID,
				PwCost:          pwCost,
				PwNonce:         stubPwNonce,
				Version:         version,
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
