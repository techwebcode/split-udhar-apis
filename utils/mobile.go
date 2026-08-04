package utils

// NormalizeMobile reduces a phone number to its last 10 digits, stripping country
// codes, spaces and punctuation. Numbers reach the API in a mix of formats
// ("+91 98765 43210", "09876543210", "9876543210"), so comparisons and map keys
// must go through this to avoid treating one contact as several.
func NormalizeMobile(mobile string) string {
	digits := make([]rune, 0, len(mobile))
	for _, ch := range mobile {
		if ch >= '0' && ch <= '9' {
			digits = append(digits, ch)
		}
	}
	if len(digits) >= 10 {
		return string(digits[len(digits)-10:])
	}
	return string(digits)
}

// SameMobile reports whether two numbers refer to the same subscriber once
// normalized. A blank or digitless number identifies nobody, so it never
// matches — otherwise unrelated contacts would collapse into a single entry.
func SameMobile(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	na, nb := NormalizeMobile(a), NormalizeMobile(b)
	return na != "" && na == nb
}
