package models

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rafaelespinoza/standardfile/encryption"
)

const (
	// _MinPasswordLength is the length of the shortest allowable password.
	_MinPasswordLength = 24
)

// ValidatePassword returns an error in the password is invalid. The rails
// implementation does not do any do any password validation; likely because the
// actual password is not stored in the backend at all, it is obfuscated and
// stored (in pieces) on the client and the backend stores a computed version of
// the password. However, in the very small possibility that the client does not
// always take care of this, we're going to do a minimal and simple validation
// anyways. See more at https://docs.standardnotes.org/specification/encryption.
func ValidatePassword(password string) error {
	if len(password) < _MinPasswordLength {
		return validationError{
			fmt.Errorf("password length must be >= %d", _MinPasswordLength),
		}
	}
	return nil
}

// PwGenParams is a set of authentication parameters used by the client to
// generate user passwords.
type PwGenParams struct {
	PwFunc     string `json:"pw_func"`
	PwAlg      string `json:"pw_alg"`
	PwCost     int    `json:"pw_cost"`
	PwKeySize  int    `json:"pw_key_size"`
	PwSalt     string `json:"pw_salt"`
	PwNonce    string `json:"pw_nonce"`
	Version    string `json:"version"`
	Identifier string `json:"identifier"` // should be email address
}

// MakePwGenParams constructs authentication parameters from User fields. NOTE:
// it's tempting to put this into the interactors package, but you can't because
// you'd get an import cycle.
func MakePwGenParams(u User) PwGenParams {
	var params PwGenParams

	if u.Email == "" {
		return params
	}

	params.Version = "003"
	params.PwCost = u.PwCost
	params.Identifier = u.Email

	if u.PwFunc != "" { // v1 only
		params.PwFunc = u.PwFunc
		params.PwAlg = u.PwAlg
		params.PwKeySize = u.PwKeySize
	}
	if u.PwSalt == "" { // v2 only
		nonce := u.PwNonce
		if nonce == "" {
			nonce = "a04a8fe6bcb19ba61c5c0873d391e987982fbbd4"
		}
		u.PwSalt = encryption.Salt(u.Email, nonce)
	}
	if u.PwNonce != "" { // v3 only
		params.PwNonce = u.PwNonce
	}

	params.PwSalt = u.PwSalt

	return params
}

// NewPassword helps facilitate user password changes.
type NewPassword struct {
	User
	CurrentPassword PwHash `json:"current_password"`
	NewPassword     PwHash `json:"new_password"`
}

type newPasswordJSON struct {
	User
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (np *NewPassword) UnmarshalJSON(in []byte) error {
	var j newPasswordJSON
	err := json.Unmarshal(in, &j)
	if err != nil {
		return err
	}
	np.User = j.User
	np.CurrentPassword = PwHash{Value: j.CurrentPassword}
	np.NewPassword = PwHash{Value: j.NewPassword}
	return nil
}

func (np NewPassword) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		newPasswordJSON{
			User:            np.User,
			CurrentPassword: np.CurrentPassword.Value,
			NewPassword:     np.NewPassword.Value,
		},
	)
}

// PwHash wraps a password string and keeps track of whether or not it's been
// hashed. At initialization, assume it hasn't been hashed yet.
type PwHash struct {
	Value  string
	Hashed bool
}

// Hash calls the hash function on the password. If it's already been hashed,
// then it returns the hashed value, but does not rehash.
func (p *PwHash) Hash() string {
	if p.Hashed {
		return p.Value
	}
	p.Value = Hash(p.Value)
	p.Hashed = true
	return p.Value
}

// Hash computes a sha256 checksum of the input.
func Hash(input string) string {
	return strings.Replace(
		fmt.Sprintf("% x", sha256.Sum256([]byte(input))),
		" ",
		"",
		-1,
	)
}
