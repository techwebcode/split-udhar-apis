package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	BaseModel
	FullName        string     `gorm:"size:100;not null" json:"full_name"`
	Email           string     `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Mobile          string     `gorm:"size:15;uniqueIndex;not null" json:"mobile"`
	IsVerified      bool       `gorm:"default:false" json:"is_verified"`
	MPIN            string     `gorm:"size:255" json:"-"` // bcrypt hash; wide enough for the 60-char digest
	GoogleID        string     `gorm:"size:255;index" json:"google_id,omitempty"`
	AuthProvider    string     `gorm:"size:50;default:'email'" json:"auth_provider,omitempty"`
	ProfileImageKey string     `gorm:"size:255" json:"profile_image_key,omitempty"`
	ProfileImageURL string     `gorm:"-" json:"profile_image_url,omitempty"`

	// Preferred display currency (ISO 4217, 3 characters). Defaults to INR.
	PreferredCurrency string `gorm:"size:3;default:'INR';not null" json:"preferred_currency"`

	// A 4-digit MPIN only has 10,000 combinations, so failed attempts are
	// counted and the account is locked out temporarily to make online
	// guessing impractical.
	MPINFailedAttempts int        `gorm:"default:0" json:"-"`
	MPINLockedUntil    *time.Time `json:"-"`
}

const (
	CurrencyINR = "INR"
	CurrencyUSD = "USD"
	CurrencyEUR = "EUR"
	CurrencyGBP = "GBP"
	CurrencyAED = "AED"
	CurrencyCAD = "CAD"
	CurrencyAUD = "AUD"
	CurrencySGD = "SGD"
	CurrencyJPY = "JPY"
	CurrencyCHF = "CHF"
)

var SupportedCurrencies = map[string]bool{
	CurrencyINR: true,
	CurrencyUSD: true,
	CurrencyEUR: true,
	CurrencyGBP: true,
	CurrencyAED: true,
	CurrencyCAD: true,
	CurrencyAUD: true,
	CurrencySGD: true,
	CurrencyJPY: true,
	CurrencyCHF: true,
}

func IsValidCurrency(code string) bool {
	return SupportedCurrencies[code]
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.PreferredCurrency == "" {
		u.PreferredCurrency = CurrencyINR
	}
	return nil
}

