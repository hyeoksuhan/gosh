package cmd

import (
  "os"
  "os/exec"
  "os/signal"
  "syscall"
  "fmt"
  "encoding/json"

  "github.com/spf13/cobra"
  "github.com/AlecAivazis/survey/v2"
  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/ssm"
  "github.com/aws/aws-sdk-go/service/ec2"
)

func init() {
  RootCmd.AddCommand(cmdStart)
}

var cmdStart = &cobra.Command{
  Use: "start",
  Short: "Start session with SSM",
  Long: "Start session with SSM",
  PreRun: func(cmd *cobra.Command, args []string) {
    handleSurveyError(setProfile())
  },
  Run: func(cmd *cobra.Command, args []string) {
    sess, err := newSession()

    if err != nil {
      panic(err)
    }

    instances := getRunningInstances(sess)

    instanceOptions := []string{}

    for name := range(instances) {
      instanceOptions = append(instanceOptions, name)
    }

    selectedInstanceOpt, err := selectInstanceId(instanceOptions)
    handleSurveyError(err)

    region := awsOpts.region
    profile := awsOpts.profile
    target := instances[selectedInstanceOpt]

    if err := startSession(sess, region, profile, target); err != nil {
      println("run command error:", err.Error())
    }
  },
}

func getRunningInstances(sess *session.Session) map[string]string {
  svc := ec2.New(sess)

  desc, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
    Filters: []*ec2.Filter{
      {
        Name: aws.String("instance-state-name"),
        Values: []*string{aws.String("running")},
      },
    },
  })

  if err != nil {
    panic(err)
  }

  instances := make(map[string]string)

  for _, reservation := range desc.Reservations {
    for _, instance := range reservation.Instances {
      name := ""
      for _, tag := range instance.Tags {
        if *tag.Key == "Name" {
          name = *tag.Value
          break
        }
      }

      key := fmt.Sprintf("%s  (%s)", name, *instance.InstanceId)
      instances[key] = *instance.InstanceId } }

  return instances
}

func selectInstanceId(instanceOpts []string) (selectedInstanceOpt string, err error) {
  selectedInstanceOpt = ""

  err = survey.AskOne(&survey.Select{
    Message: "Instance:",
    Options: instanceOpts,
    VimMode: true,
  }, &selectedInstanceOpt, survey.WithPageSize(20))

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

func startSession(sess *session.Session, region, profile, instanceId string) error {
  // ignore ctrl+c
  sigch := make(chan os.Signal)
  signal.Notify(sigch, syscall.SIGINT)
  defer close(sigch)

  svc := ssm.New(sess)

  input := &ssm.StartSessionInput{
    Target: &instanceId,
  }

  sessOutput, err := svc.StartSession(input)
  if err != nil {
    panic(err)
  }

  outputJson, err := json.Marshal(sessOutput)
  if err != nil {
    panic(err)
  }

  inputJson, err := json.Marshal(input)
  if err != nil {
    panic(err)
  }

  err = runCommand("session-manager-plugin",
    string(outputJson),
    region,
    "StartSession",
    profile,
    string(inputJson),
    svc.Endpoint,
  )

  println("finished session")

  return err
}

