package models

import (
	"fmt"
	"regexp"
)

const (
	// _MinEmailLength is the shortest allowable length for an email address.
	// If it was an intranet address, then it'd be x@y, so 3. But nobody has
	// complained about that so far. Until then, set the shortest email length
	// to the shortest possible externally-facing email: `a@b.cd`, so 6.
	_MinEmailLength = 6
	_MaxEmailLength = 255

	// MinIDLength is lower length limit for a stringified v4 UUID. The format
	// would be: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.
	MinIDLength = 36
)

type validationError struct{ error }

func (e validationError) Validation() bool { return true }

// ValidateEmail returns an error in the email address is invalid.
func ValidateEmail(email string) error {
	if len(email) < _MinEmailLength {
		return validationError{
			fmt.Errorf("email invalid, length must be >= %d", _MinEmailLength),
		}
	} else if len(email) > _MaxEmailLength {
		return validationError{
			fmt.Errorf("email invalid, length must be <= %d", _MaxEmailLength),
		}
	}

	match, err := regexp.MatchString(`^.+@.+$`, email)
	if err != nil {
		return err
	}
	if !match {
		return validationError{fmt.Errorf("email invalid")}
	}
	return nil
}
