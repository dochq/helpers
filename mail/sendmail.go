package http

import (
	"bytes"
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
	Buffer *bytes.Buffer
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
			Dsn:              "https://93478dd3044948559fb5e2d23a821d40@o239521.ingest.sentry.io/6033959",
			AttachStacktrace: true,
		}); err != nil {
			panic(err)
		}

		// Enable the console logger and sentry logger
		logging.Logger = append(logging.Logger, &sentry.Logger{IgnoreLevelBelow: 2})
	}

	sendgridClient = sendgrid.NewSendClient(os.Getenv("SENDGRID_API_KEY"))
}

func SendGridEmail(sendGridEmailTmpl string, fromEmail *mail.Email, receipients []string, subject, body string, fileInfo []*FileInfo) error {
	var peopleToEmail []*mail.Email

	for _, receipient := range receipients {
		peopleToEmail = append(peopleToEmail, &mail.Email{
			Address: receipient,
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
			},
		},
	}

	for _, fl := range fileInfo {
		sendData.AddAttachment(&mail.Attachment{
			Content:     base64.StdEncoding.EncodeToString(fl.Buffer.Bytes()),
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
