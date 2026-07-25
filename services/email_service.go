package services

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
)

type EmailService struct{}

func NewEmailService() *EmailService {
	return &EmailService{}
}

func (s *EmailService) SendOTP(email, otp string) error {

	from := os.Getenv("SMTP_EMAIL")
	password := os.Getenv("SMTP_PASSWORD")
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")

	auth := smtp.PlainAuth("", from, password, host)

	subject := "Subject: Login OTP\r\n"
	body := fmt.Sprintf("Your OTP is %s.\nIt is valid for 5 minutes.", otp)

	message := []byte(subject + "\r\n" + body)

	log.Print(from, password, host, port)

	return smtp.SendMail(
		host+":"+port,
		auth,
		from,
		[]string{email},
		message,
	)
}
