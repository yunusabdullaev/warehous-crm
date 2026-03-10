package auth

import (
	"errors"
	"unicode"
)

const MinPasswordLength = 10

// ValidatePassword enforces: min 10 chars, require 3 of 4 (upper/lower/digit/symbol).
func ValidatePassword(pw string) error {
	if len(pw) < MinPasswordLength {
		return errors.New("password must be at least 10 characters")
	}

	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, r := range pw {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		default:
			hasSymbol = true
		}
	}

	score := 0
	if hasUpper {
		score++
	}
	if hasLower {
		score++
	}
	if hasDigit {
		score++
	}
	if hasSymbol {
		score++
	}

	if score < 3 {
		return errors.New("password must contain at least 3 of: uppercase, lowercase, digit, symbol")
	}

	return nil
}
