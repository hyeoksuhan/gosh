package cmd

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

const (
	defaultRegion  = "ap-northeast-2"
	defaultProfile = "default"
)

var (
	rootCmd = &cobra.Command{
		Use:   "gosh",
		Short: "gosh is a tool to use SSM easily",
	}

	awsOpts = AWSopts{}
)

// AWSopts contains region, profile
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

// Execute rootCmd
func Execute() {
	rootCmd.Execute()
}
