package actor

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Typed complements ("contacts") are validated and normalized per type so a
// phone is a phone and an IDE carries a valid check digit; the SPA mirrors
// these rules for immediate feedback (utils/contactRules.ts), but this file is
// the authority. Values are stored in their normalized form.

var (
	phoneSeparators = strings.NewReplacer(" ", "", "-", "", ".", "", "/", "", "(", "", ")", "")
	e164Pattern     = regexp.MustCompile(`^\+[1-9]\d{6,14}$`)
	postalBoxRe     = regexp.MustCompile(`(?i)^(?:case postale|c\.?p\.?|postfach|pf|po box|p\.o\. box)?\s*(\d{1,6})$`)
	ideDigitsRe     = regexp.MustCompile(`^CHE(\d{9})$`)
	vatRe           = regexp.MustCompile(`(?i)^(.+?)\s*(MWST|TVA|IVA)$`)
	abacusRe        = regexp.MustCompile(`^\d{1,10}$`)
	registerIDRe    = regexp.MustCompile(`^CH(\d{3})(\d)(\d{3})(\d{3})(\d)$`)
	idSeparators    = strings.NewReplacer(" ", "", ".", "", "-", "")
	ideCheckWeights = [8]int{5, 4, 3, 2, 7, 6, 5, 4}
)

// maxEmailLength is the RFC 5321 path limit.
const maxEmailLength = 254

// contactNormalizers maps each type to its validator/normalizer; OTHER keeps
// free text (its label is required instead).
var contactNormalizers = map[ContactType]func(string) (string, error){
	ContactTypePhone:              normalizePhone,
	ContactTypePhonePrivate:       normalizePhone,
	ContactTypePhonePro:           normalizePhone,
	ContactTypeMobile:             normalizePhone,
	ContactTypeFax:                normalizePhone,
	ContactTypeEmail:              normalizeEmail,
	ContactTypeWebsite:            normalizeWebsite,
	ContactTypePostalBox:          normalizePostalBox,
	ContactTypeIDEFederal:         normalizeIDE,
	ContactTypeVATNumber:          normalizeVAT,
	ContactTypeABACUSDebtor:       normalizeABACUS,
	ContactTypeCommercialRegister: normalizeCommercialRegister,
}

// NormalizeContactValue validates value for contactType and returns its stored
// form, or a core.ErrInvalidInput error explaining the expected format.
func NormalizeContactValue(contactType ContactType, value string) (string, error) {
	value = strings.TrimSpace(value)
	normalize, ok := contactNormalizers[contactType]
	if !ok {
		return value, nil
	}
	normalized, err := normalize(value)
	if err != nil {
		return "", fmt.Errorf("%w: %s: %w", core.ErrInvalidInput, contactType, err)
	}
	return normalized, nil
}

// normalizePhone returns the E.164 form: separators are dropped, a 00 prefix
// becomes +, and a 10-digit Swiss national number (0xx xxx xx xx) gets +41.
func normalizePhone(v string) (string, error) {
	n := phoneSeparators.Replace(v)
	switch {
	case strings.HasPrefix(n, "00"):
		n = "+" + n[2:]
	case len(n) == 10 && strings.HasPrefix(n, "0"):
		n = "+41" + n[1:]
	}
	if !e164Pattern.MatchString(n) {
		return "", fmt.Errorf("expected an international (+41 21 315 22 22) or Swiss (021 315 22 22) number")
	}
	return n, nil
}

// normalizeEmail accepts a bare address (no display name) with a dotted domain,
// lower-casing the domain.
func normalizeEmail(v string) (string, error) {
	addr, err := mail.ParseAddress(v)
	if err != nil || addr.Name != "" || addr.Address != v || len(v) > maxEmailLength {
		return "", fmt.Errorf("expected an e-mail address such as name@example.ch")
	}
	at := strings.LastIndex(v, "@")
	domain := strings.ToLower(v[at+1:])
	if !strings.Contains(domain, ".") {
		return "", fmt.Errorf("expected an e-mail address such as name@example.ch")
	}
	return v[:at+1] + domain, nil
}

// normalizeWebsite accepts an http(s) URL with a dotted host, adding https://
// when the scheme is missing.
func normalizeWebsite(v string) (string, error) {
	if !strings.Contains(v, "://") {
		v = "https://" + v
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !strings.Contains(u.Hostname(), ".") || u.User != nil {
		return "", fmt.Errorf("expected a web address such as https://www.example.ch")
	}
	u.Host = strings.ToLower(u.Host)
	return u.String(), nil
}

// normalizePostalBox accepts "Case postale 123", "CP 123", "Postfach 123" or
// "123" and stores "Case postale 123".
func normalizePostalBox(v string) (string, error) {
	m := postalBoxRe.FindStringSubmatch(v)
	if m == nil {
		return "", fmt.Errorf("expected a postal box number such as Case postale 1234")
	}
	return "Case postale " + m[1], nil
}

// normalizeIDE accepts CHE-123.456.788 in any spacing and checks the modulo-11
// check digit of the federal business identifier (UID).
func normalizeIDE(v string) (string, error) {
	m := ideDigitsRe.FindStringSubmatch(strings.ToUpper(idSeparators.Replace(v)))
	if m == nil || !validIDECheckDigit(m[1]) {
		return "", fmt.Errorf("expected a valid IDE such as CHE-123.456.788 (check digit included)")
	}
	d := m[1]
	return "CHE-" + d[0:3] + "." + d[3:6] + "." + d[6:9], nil
}

// validIDECheckDigit applies the UID rule: weights 5,4,3,2,7,6,5,4 on the first
// eight digits, check = 11 - sum mod 11 (11 → 0; 10 is never issued).
func validIDECheckDigit(digits string) bool {
	sum := 0
	for i, w := range ideCheckWeights {
		sum += int(digits[i]-'0') * w
	}
	check := 11 - sum%11
	if check == 11 {
		check = 0
	}
	return check != 10 && check == int(digits[8]-'0')
}

// normalizeVAT accepts an IDE followed by MWST, TVA or IVA and stores
// "CHE-123.456.788 TVA".
func normalizeVAT(v string) (string, error) {
	m := vatRe.FindStringSubmatch(v)
	if m == nil {
		return "", fmt.Errorf("expected a VAT number such as CHE-123.456.788 TVA")
	}
	ide, err := normalizeIDE(m[1])
	if err != nil {
		return "", fmt.Errorf("expected a VAT number such as CHE-123.456.788 TVA")
	}
	return ide + " " + strings.ToUpper(m[2]), nil
}

// normalizeABACUS accepts a debtor number of 1 to 10 digits (spaces ignored).
func normalizeABACUS(v string) (string, error) {
	n := strings.ReplaceAll(v, " ", "")
	if !abacusRe.MatchString(n) {
		return "", fmt.Errorf("expected a debtor number of 1 to 10 digits")
	}
	return n, nil
}

// normalizeCommercialRegister accepts a register URL or a register identifier
// such as CH-550.1.012.345-6, stored as CH-ddd.d.ddd.ddd-d.
func normalizeCommercialRegister(v string) (string, error) {
	if m := registerIDRe.FindStringSubmatch(strings.ToUpper(idSeparators.Replace(v))); m != nil {
		return fmt.Sprintf("CH-%s.%s.%s.%s-%s", m[1], m[2], m[3], m[4], m[5]), nil
	}
	if strings.Contains(v, "://") {
		if u, err := normalizeWebsite(v); err == nil {
			return u, nil
		}
	}
	return "", fmt.Errorf("expected a register identifier such as CH-550.1.012.345-6 or a register web address")
}
