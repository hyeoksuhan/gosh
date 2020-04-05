package cmd

import (
  "github.com/spf13/cobra"
  "github.com/AlecAivazis/survey/v2"
)

const (
  defaultRegion = "ap-northeast-2"
  defaultProfile = "default"
)

var (
  rootCmd = &cobra.Command{
    Use: "gosh",
    Short: "gosh is a tool to use SSM easily",
  }

  awsOpts = AWSopts{}
)

type AWSopts struct {
  region, profile string
}

func init() {
  rootCmd.PersistentFlags().StringVarP(&awsOpts.region, "region", "r", defaultRegion, "AWS Region")
  rootCmd.PersistentFlags().StringVarP(&awsOpts.profile, "profile", "p", "", "AWS profile name")
}

func setProfile() error {
  if awsOpts.profile != "" {
    return nil
  }

  return survey.AskOne(&survey.Input{
    Message: "Profile name:",
    Default: defaultProfile,
  }, &awsOpts.profile)
}

func Execute() {
  rootCmd.Execute()
}

