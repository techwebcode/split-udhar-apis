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
	from := os.Getenv("SMTP_EMAIL")
	password := os.Getenv("SMTP_PASSWORD")
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")

	if host == "" {
		host = "smtp.gmail.com"
	}
	if port == "" {
		port = "587"
	}

	htmlBody, err := utils.ParseOTPTemplate(templateFile, map[string]string{"OTP": otp})
	if err != nil || htmlBody == "" {
		log.Printf("[EMAIL SERVICE WARNING] Template parse error for %s: %v", templateFile, err)
		htmlBody = fmt.Sprintf("<h2>%s</h2><p>Your OTP code is: <strong>%s</strong> (valid for 10 minutes).</p>", subjectTitle, otp)
	}

	htmlBody = strings.ReplaceAll(htmlBody, "{{OTP}}", otp)
	htmlBody = strings.ReplaceAll(htmlBody, "{{.OTP}}", otp)

	subjectHeader := fmt.Sprintf("Subject: %s\r\n", subjectTitle)
	fromHeader := fmt.Sprintf("From: %s\r\n", from)
	toHeader := fmt.Sprintf("To: %s\r\n", email)
	mimeHeader := "MIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n"

	msg := []byte(fromHeader + toHeader + subjectHeader + mimeHeader + htmlBody)

	auth := smtp.PlainAuth("", from, password, host)

	log.Printf("[EMAIL SERVICE] Dispatching email [%s] to %s via %s:%s", subjectTitle, email, host, port)

	return smtp.SendMail(
		host+":"+port,
		auth,
		from,
		[]string{email},
		msg,
	)
}
