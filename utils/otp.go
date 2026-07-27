package utils

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"text/template"
)

func GenerateOTP() string {
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

// ParseOTPTemplate resolves the template file from disk, executing Go text/template or doing string replacement.
func ParseOTPTemplate(templateName string, data any) (string, error) {
	candidates := []string{
		templateName,
		"templates/" + templateName,
		"templates/forgot-password-otp.html",
		"templates/signUp-otp.html",
		"../templates/" + templateName,
		"../../templates/" + templateName,
		"../../../templates/" + templateName,
	}

	var resolvedPath string
	for _, p := range candidates {
		if p != "" {
			if _, err := os.Stat(p); err == nil {
				resolvedPath = p
				break
			}
		}
	}

	if resolvedPath != "" {
		tmpl, err := template.ParseFiles(resolvedPath)
		if err == nil {
			var body bytes.Buffer
			if err := tmpl.Execute(&body, data); err == nil {
				res := body.String()
				if m, ok := data.(map[string]string); ok && m["OTP"] != "" {
					res = strings.ReplaceAll(res, "{{OTP}}", m["OTP"])
				}
				return res, nil
			}
		}
	}

	otpVal := ""
	if m, ok := data.(map[string]string); ok {
		otpVal = m["OTP"]
	}

	fallbackHTML := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><title>SplitUdhar OTP</title></head>
<body style="font-family:Arial,sans-serif;background:#f4f7fb;padding:30px;">
  <div style="max-width:500px;margin:auto;background:#fff;padding:30px;border-radius:10px;">
    <h2 style="color:#0F766E;">SplitUdhar - Email Verification</h2>
    <p>Use the following OTP code to verify your account:</p>
    <div style="font-size:32px;font-weight:bold;color:#0F766E;background:#f1fffc;padding:15px;text-align:center;border-radius:8px;letter-spacing:6px;margin:20px 0;">%s</div>
    <p>This code is valid for 10 minutes. Do not share this OTP with anyone.</p>
  </div>
</body>
</html>`, otpVal)

	return fallbackHTML, nil
}
