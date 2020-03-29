package cmd

import (
  "os"
  "os/exec"
  "os/signal"
  "syscall"
  "fmt"

  "github.com/spf13/cobra"
  "github.com/AlecAivazis/survey/v2"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/aws"
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
    setProfile()
  },
  Run: func(cmd *cobra.Command, args []string) {
    sess, err := newSession()

    if err != nil {
      println(err.Error())
      panic(err)
    }

    instances := getRunningInstances(sess)
    instanceId := askInstanceId(instances)

    startSession(instanceId)
  },
}

func getRunningInstances(sess *session.Session) map[string]string {
  svc := ec2.New(sess)
  input := &ec2.DescribeInstancesInput{
    Filters: []*ec2.Filter{
      {
        Name: aws.String("instance-state-name"),
        Values: []*string{aws.String("running")},
      },
    },
  }

  result, err := svc.DescribeInstances(input)
  if err != nil {
    println(err.Error())
    panic(err)
  }

  instances := make(map[string]string)

  for _, reservation := range result.Reservations {
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

func askInstanceId(instances map[string]string) string {
  var instanceOptions []string

  for name := range(instances) {
    instanceOptions = append(instanceOptions, name)
  }

  selectedInstance := ""

  prompt := &survey.Select{
    Message: "Instance:",
    Options: instanceOptions,
    VimMode: true,
  }

  survey.AskOne(prompt, &selectedInstance, survey.WithPageSize(20))

  instanceId := instances[selectedInstance]

  return instanceId
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

func startSession(instanceId string) {
  profile := awsOpts.profile

  // ignore ctrl+c
  sigch := make(chan os.Signal)
  signal.Notify(sigch, syscall.SIGINT)
  defer close(sigch)

  err := runCommand("aws", "ssm", "start-session", "--profile", profile, "--target", instanceId)
  println("finished session successfully")

  if err != nil {
    println("run command error", err)
  }
}

