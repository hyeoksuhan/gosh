package cmd

import (
  "github.com/spf13/cobra"
  "github.com/AlecAivazis/survey/v2"
)

const defaultRegion = "ap-northeast-2"
const defaultProfile = "default"

var (
  RootCmd = &cobra.Command{
    Use: "gosh",
    Short: "gosh is a tool to use SSM easily",
  }

  awsOpts = AWSopts{}
)

type AWSopts struct {
  region, profile string
}

func init() {
  RootCmd.PersistentFlags().StringVarP(&awsOpts.region, "region", "r", defaultRegion, "AWS Region")
  RootCmd.PersistentFlags().StringVarP(&awsOpts.profile, "profile", "p", "", "AWS profile name")
}

func setProfile() {
  if awsOpts.profile == "" {
    prompt := &survey.Input{
      Message: "Profile name:",
      Default: defaultProfile,
    }

    survey.AskOne(prompt, &awsOpts.profile)
  }
}

func Execute() {
  RootCmd.Execute()
}

