package models

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rafaelespinoza/standardnotes/encryption"
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

// MakePwGenParams constructs authentication parameters from User fields.
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

// PwChangeParams helps facilitate user password changes.
type PwChangeParams struct {
	CurrentPassword PwHash `json:"current_password"`
	NewPassword     PwHash `json:"new_password"`
	API             string `json:"api"`
	Identifier      string `json:"identifier"`
	PwCost          int    `json:"pw_cost"`
	PwNonce         string `json:"pw_nonce"`
	Version         string `json:"version"`
}

type jsonPwChangeParams struct {
	CurrPassword string `json:"current_password"`
	NextPassword string `json:"new_password"`
	API          string `json:"api"`
	Identifier   string `json:"identifier"`
	PwCost       int    `json:"pw_cost"`
	PwNonce      string `json:"pw_nonce"`
	Version      string `json:"version"`
}

func (np *PwChangeParams) UnmarshalJSON(in []byte) error {
	var j jsonPwChangeParams
	err := json.Unmarshal(in, &j)
	if err != nil {
		return err
	}
	np.CurrentPassword = PwHash{Value: j.CurrPassword}
	np.NewPassword = PwHash{Value: j.NextPassword}
	np.API = j.API
	np.Identifier = j.Identifier
	np.PwCost = j.PwCost
	np.PwNonce = j.PwNonce
	np.Version = j.Version
	return nil
}

func (np PwChangeParams) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		jsonPwChangeParams{
			CurrPassword: np.CurrentPassword.Value,
			NextPassword: np.NewPassword.Value,
			API:          np.API,
			Identifier:   np.Identifier,
			PwCost:       np.PwCost,
			PwNonce:      np.PwNonce,
			Version:      np.Version,
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
