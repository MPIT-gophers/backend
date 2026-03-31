package validation

import "net/mail"

func IsEmail(value string) bool {
	_, err := mail.ParseAddress(value)
	return err == nil
}
