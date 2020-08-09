package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/hyeoksuhan/gosh/service/gossm"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cmdStart)
}

var cmdStart = &cobra.Command{
	Use:   "start",
	Short: "Start session with SSM",
	Long:  "Start session with SSM",
	PreRun: func(cmd *cobra.Command, args []string) {
		if err := setProfile(); err != nil {
			fatal(err)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		svc, err := gossm.New(awsOpts.region, awsOpts.profile)
		if err != nil {
			fatal(err)
		}

		target, err := getTarget(svc.Session)
		if err != nil {
			fatal(err)
		}

		sid, err := startSession(svc, target)

		defer func(sid string) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if sid == "" {
				return
			}

			if err := svc.TerminateSession(ctx, sid); err != nil {
				panic(err)
			}

			print("terminated session")
		}(sid)

		if err != nil {
			panic(err)
		}
	},
}

func getRunningInstances(sess *session.Session) (result map[string]string, err error) {
	result = make(map[string]string)

	svc := ec2.New(sess)

	desc, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("instance-state-name"),
				Values: []*string{aws.String("running")},
			},
		},
	})

	if err != nil {
		return
	}

	for _, r := range desc.Reservations {
		for _, instance := range r.Instances {
			name := ""
			for _, tag := range instance.Tags {
				if *tag.Key == "Name" {
					name = *tag.Value
					break
				}
			}

			key := fmt.Sprintf("%s  (%s)", name, *instance.InstanceId)
			result[key] = *instance.InstanceId
		}
	}

	return
}

func getTarget(sess *session.Session) (string, error) {
	instances, err := getRunningInstances(sess)
	if err != nil {
		return "", err
	}

	names := []string{}
	for name := range instances {
		names = append(names, name)
	}

	selected, err := selectInstance(names)
	if err != nil {
		return "", err
	}

	return instances[selected], nil
}

func selectInstance(names []string) (result string, err error) {
	result = ""

	err = survey.AskOne(&survey.Select{
		Message: "Instance:",
		Options: names,
		VimMode: true,
	}, &result, survey.WithPageSize(20))

	return
}

func runCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func startSession(svc gossm.SSMservice, target string) (sid string, err error) {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT)
	defer close(ch)

	output, err := svc.StartSession(&ssm.StartSessionInput{
		Target: &target,
	})
	if err != nil {
		return
	}

	sid = output.SessionID
	err = runCommand(output.Command, output.CommandArgs...)

	println("finished session")

	return
}
