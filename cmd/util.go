package cmd

import (
  "os"

  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/aws"
  "github.com/AlecAivazis/survey/v2/terminal"
  "github.com/fatih/color"
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
    os.Exit(0)
  }

  if err != nil {
    panic(err)
  }
}

func getColorFunc(i int) func(...interface {}) string {
  attributes := []color.Attribute{
    color.FgRed,
    color.FgGreen,
    color.FgYellow,
    color.FgBlue,
    color.FgMagenta,
    color.FgCyan,
    color.FgWhite,
    color.BgRed,
    color.BgGreen,
    color.BgYellow,
    color.BgBlue,
    color.BgMagenta,
    color.BgCyan,
    color.BgWhite,
  }

  selected := attributes[i % len(attributes)]

  return color.New(selected).SprintFunc()
}

