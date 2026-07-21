package app

import (
	"net/mail"
	"strings"
	"unicode"
)

type fieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}
	return address.Address == email && strings.Contains(email, "@")
}

func validatePassword(password string) bool {
	if len(password) < 8 {
		return false
	}

	var hasLetter bool
	var hasNumber bool
	var hasUpper bool
	for _, r := range password {
		if unicode.IsLetter(r) {
			hasLetter = true
			if unicode.IsUpper(r) {
				hasUpper = true
			}
		}
		if unicode.IsDigit(r) {
			hasNumber = true
		}
	}

	return hasLetter && hasNumber && hasUpper
}

func defaultNameFromEmail(email string) string {
	local, _, ok := strings.Cut(email, "@")
	if !ok || strings.TrimSpace(local) == "" {
		return "user"
	}
	return local
}
