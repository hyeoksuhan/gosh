package cmd

import (
  "os"
  "fmt"

  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/aws"
  "github.com/AlecAivazis/survey/v2/terminal"
)

func newSession() (*session.Session, error) {
  region := awsOpts.region
  profile := awsOpts.profile

  return session.NewSessionWithOptions(session.Options{
    Profile: profile,
    Config: aws.Config{
      Region: aws.String(region),
    },
    SharedConfigState: session.SharedConfigEnable,
  })
}

func handleSurveyError(err error) {
  if err == terminal.InterruptErr {
    fmt.Println("Interrupted")

    os.Exit(0)
  }

  if err != nil {
    panic(err)
  }
}

