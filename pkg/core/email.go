package core

import (
	"errors"
	"net/mail"
	"strings"
)

// MaxEmailLength is the RFC 5321 path limit.
const MaxEmailLength = 254

// errEmailFormat is the message of every rejected address.
var errEmailFormat = errors.New("expected an e-mail address such as name@example.ch")

// NormalizeEmail accepts a bare address (no display name) with a dotted
// domain, lower-casing the domain; the local part is kept as given.
func NormalizeEmail(v string) (string, error) {
	addr, err := mail.ParseAddress(v)
	if err != nil || addr.Name != "" || addr.Address != v || len(v) > MaxEmailLength {
		return "", errEmailFormat
	}
	at := strings.LastIndex(v, "@")
	domain := strings.ToLower(v[at+1:])
	if !strings.Contains(domain, ".") {
		return "", errEmailFormat
	}
	return v[:at+1] + domain, nil
}
