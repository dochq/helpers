package http

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"

	"github.com/DocHQ/logging"
	"github.com/DocHQ/logging/sentry"
)

type FileInfo struct {
	Name   string
	Type   string
	Buffer []byte
}

var sendgridClient *sendgrid.Client

func init() {
	// Init all our logging stuff
	// Init sentry and connect to DSN
	if os.Getenv("DEBUG") == "true" {
		logging.Verbose = true
	}

	// Only enable sentry on production
	if os.Getenv("ENVIRONMENT") == "production" {
		if err := sentry.InitSentry(&sentry.ConfigOptions{
			Dsn:              os.Getenv("SENDGRID_API_URL"),
			AttachStacktrace: true,
		}); err != nil {
			panic(err)
		}

		// Enable the console logger and sentry logger
		logging.Logger = append(logging.Logger, &sentry.Logger{IgnoreLevelBelow: 2})
	}

	sendgridClient = sendgrid.NewSendClient(os.Getenv("SENDGRID_API_KEY"))
}

func SendGridEmail(sendGridEmailTmpl string, fromEmail *mail.Email, receipients []*mail.Email, subject, body string, fileInfo []*FileInfo) error {
	var peopleToEmail []*mail.Email

	for _, receipient := range receipients {
		peopleToEmail = append(peopleToEmail, &mail.Email{
			Name:    receipient.Name,
			Address: receipient.Address,
		})
	}

	sendData := &mail.SGMailV3{
		TemplateID: sendGridEmailTmpl,
		Subject:    subject,
		From: &mail.Email{
			Name:    fromEmail.Name,
			Address: fromEmail.Address,
		},
		Personalizations: []*mail.Personalization{
			{
				To: peopleToEmail,
				Substitutions: map[string]string{
					"email_body": body,
				},
			},
		},
	}

	for _, fl := range fileInfo {
		sendData.AddAttachment(&mail.Attachment{
			Content:     base64.StdEncoding.EncodeToString(fl.Buffer),
			Type:        fl.Type,
			Filename:    fl.Name,
			Disposition: "attachment",
		})
	}

	res, err := sendgridClient.Send(sendData)
	if err != nil {
		return err
	}

	if res.StatusCode != 202 {
		logging.Debugf("Error: %v ", res.Body)
		return fmt.Errorf("incorrect status code reurned %v", res.StatusCode)
	}

	return err
}
