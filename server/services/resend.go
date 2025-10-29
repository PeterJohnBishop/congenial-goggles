package services

import (
	"log"
	"os"

	"github.com/resend/resend-go/v2"
)

func InitResendClient() *resend.Client {
	apiKey := os.Getenv("RESEND_API_KEY")
	return resend.NewClient(apiKey)
}

func SendURL(client *resend.Client, toEmail []string, presignedURL string) error {
	// Implementation for sending email via Resend API
	params := &resend.SendEmailRequest{
		From:    "Acme <onboarding@peterjohnbishop.com>",
		To:      toEmail,
		Html:    "<strong>hello world</strong>",
		Subject: "Hello from Golang",
		Cc:      []string{""},
		Bcc:     []string{""},
		ReplyTo: "replyto@example.com",
	}

	sent, err := client.Emails.Send(params)
	if err != nil {
		return err
	}
	log.Printf("Email sent with ID: %s", sent.Id)
	return nil
}

func SendQR(client *resend.Client, toEmail, qrCodeData string) error {
	// Implementation for sending QR code via Resend API
	params := &resend.SendEmailRequest{
		From:    "Acme <onboarding@resend.dev>",
		To:      []string{"delivered@resend.dev"},
		Html:    "<strong>hello world</strong>",
		Subject: "Hello from Golang",
		Cc:      []string{"cc@example.com"},
		Bcc:     []string{"bcc@example.com"},
		ReplyTo: "replyto@example.com",
	}

	sent, err := client.Emails.Send(params)
	if err != nil {
		return err
	}
	log.Printf("Email sent with ID: %s", sent.Id)
	return nil
}
