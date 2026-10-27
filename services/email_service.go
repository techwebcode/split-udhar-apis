package services

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
	"split-udhar-apis/utils"
	"strings"
)

type EmailService struct{}

func NewEmailService() *EmailService {
	return &EmailService{}
}

func (s *EmailService) SendOTP(email, otp string) error {
	return s.sendMailWithTemplate(email, otp, "SplitUdhar - Email Verification OTP", "signUp-otp.html")
}

func (s *EmailService) SendForgotPasscodeOTP(email, otp string) error {
	return s.sendMailWithTemplate(email, otp, "SplitUdhar - Passcode Reset OTP", "forgot-password-otp.html")
}

func (s *EmailService) sendMailWithTemplate(email, otp, subjectTitle, templateFile string) error {
	// 1. SMTP Auth credentials
	smtpUser := os.Getenv("SMTP_USER")
	if smtpUser == "" {
		smtpUser = os.Getenv("SMTP_EMAIL") // Fallback if using old key name
	}
	password := os.Getenv("SMTP_PASSWORD")
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")

	if host == "" {
		host = "smtp-relay.brevo.com"
	}
	if port == "" {
		port = "587"
	}

	// 2. Verified Sender details (from Brevo verified senders)
	fromEmail := os.Getenv("FROM_EMAIL")
	if fromEmail == "" {
		fromEmail = "auth-splitudhar@techwebcode.in"
	}
	fromName := os.Getenv("FROM_NAME")
	if fromName == "" {
		fromName = "SplitUdhar"
	}

	htmlBody, err := utils.ParseOTPTemplate(templateFile, map[string]string{"OTP": otp})
	if err != nil || htmlBody == "" {
		log.Printf("[EMAIL SERVICE WARNING] Template parse error for %s: %v", templateFile, err)
		htmlBody = fmt.Sprintf("<h2>%s</h2><p>Your OTP code is: <strong>%s</strong> (valid for 10 minutes).</p>", subjectTitle, otp)
	}

	htmlBody = strings.ReplaceAll(htmlBody, "{{OTP}}", otp)
	htmlBody = strings.ReplaceAll(htmlBody, "{{.OTP}}", otp)

	// 3. Proper RFC Headers
	subjectHeader := fmt.Sprintf("Subject: %s\r\n", subjectTitle)
	fromHeader := fmt.Sprintf("From: %s <%s>\r\n", fromName, fromEmail)
	toHeader := fmt.Sprintf("To: %s\r\n", email)
	mimeHeader := "MIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n"

	msg := []byte(fromHeader + toHeader + subjectHeader + mimeHeader + htmlBody)

	// 4. Authenticate using SMTP credentials
	auth := smtp.PlainAuth("", smtpUser, password, host)

	log.Printf("[EMAIL SERVICE] Dispatching email [%s] from %s to %s via %s:%s", subjectTitle, fromEmail, email, host, port)

	// 5. Send with envelope sender set to fromEmail
	return smtp.SendMail(
		host+":"+port,
		auth,
		fromEmail, // Must be your verified domain email!
		[]string{email},
		msg,
	)
}
