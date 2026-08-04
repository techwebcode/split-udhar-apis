package utils

import "testing"

func TestNormalizeMobile(t *testing.T) {
	cases := map[string]string{
		"9876543210":        "9876543210",
		"+919876543210":     "9876543210",
		"+91 98765 43210":   "9876543210",
		"09876543210":       "9876543210",
		"(+91)-98765-43210": "9876543210",
		"12345":             "12345", // too short to trim, returned as digits
		"":                  "",
		"abc":               "",
	}

	for in, want := range cases {
		if got := NormalizeMobile(in); got != want {
			t.Errorf("NormalizeMobile(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSameMobile(t *testing.T) {
	same := [][2]string{
		{"9876543210", "+919876543210"},
		{"+91 98765 43210", "09876543210"},
		{"9876543210", "9876543210"},
	}
	for _, pair := range same {
		if !SameMobile(pair[0], pair[1]) {
			t.Errorf("SameMobile(%q, %q) = false, want true", pair[0], pair[1])
		}
	}

	different := [][2]string{
		{"9876543210", "9876543211"},
		{"9876543210", ""},
		{"", "9876543210"},
		{"abc", "def"}, // no digits on either side
		{"", ""},       // neither identifies a contact
	}
	for _, pair := range different {
		if SameMobile(pair[0], pair[1]) {
			t.Errorf("SameMobile(%q, %q) = true, want false", pair[0], pair[1])
		}
	}
}

func TestHashAndCheckMPIN(t *testing.T) {
	hashed, err := HashMPIN("1234")
	if err != nil {
		t.Fatalf("HashMPIN returned error: %v", err)
	}

	if hashed == "1234" {
		t.Fatal("HashMPIN returned the MPIN in plaintext")
	}
	if !IsHashedMPIN(hashed) {
		t.Errorf("IsHashedMPIN(%q) = false, want true", hashed)
	}
	if !CheckMPIN(hashed, "1234") {
		t.Error("CheckMPIN should accept the correct MPIN")
	}
	if CheckMPIN(hashed, "4321") {
		t.Error("CheckMPIN should reject an incorrect MPIN")
	}
}

// Accounts predating hashing still hold plaintext and must keep working, so the
// owner can log in once and be upgraded.
func TestCheckMPINAcceptsLegacyPlaintext(t *testing.T) {
	if !CheckMPIN("1234", "1234") {
		t.Error("legacy plaintext MPIN should still verify")
	}
	if CheckMPIN("1234", "9999") {
		t.Error("legacy plaintext MPIN should reject a wrong value")
	}
	if IsHashedMPIN("1234") {
		t.Error("plaintext should not be reported as hashed")
	}
}

func TestCheckMPINRejectsEmptyStored(t *testing.T) {
	if CheckMPIN("", "") {
		t.Error("an account with no MPIN set must never verify")
	}
	if CheckMPIN("", "1234") {
		t.Error("an account with no MPIN set must never verify")
	}
}

func TestGenerateOTPIsSixDigits(t *testing.T) {
	for i := 0; i < 200; i++ {
		otp := GenerateOTP()
		if len(otp) != 6 {
			t.Fatalf("GenerateOTP() = %q, want 6 characters", otp)
		}
		for _, ch := range otp {
			if ch < '0' || ch > '9' {
				t.Fatalf("GenerateOTP() = %q, want digits only", otp)
			}
		}
	}
}
