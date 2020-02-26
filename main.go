package main

import (
  "flag"

  "os"
  "os/exec"
  "os/signal"
  "syscall"

  "github.com/AlecAivazis/survey/v2"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/service/ec2"
)

func askProfile() string {
  profile := ""

  prompt := &survey.Input{
    Message: "Profile name:",
    Default: "default",
  }

  survey.AskOne(prompt, &profile)

  return profile
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

  survey.AskOne(prompt, &selectedInstance)

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

func startSession(profile string, instanceId string) {
  // ignore ctrl+c
  sigch := make(chan os.Signal)
  signal.Notify(sigch, syscall.SIGINT)
  defer close(sigch)

  err := runCommand("aws", "ssm", "start-session","--profile", profile, "--target", instanceId)
  println("finished session successfully")

  if err != nil {
    println("run command error", err)
  }
}

type Options struct {
  Profile *string
}

func getOptions(options *Options) {
  options.Profile = flag.String("profile", "", "profile MUST be string type")
  flag.Parse()

  if *options.Profile == "" {
    *options.Profile = askProfile()
  }
}

func main() {
  options := Options{}
  getOptions(&options)

  sess, err := session.NewSessionWithOptions(session.Options{
    Profile: *options.Profile,
    Config: aws.Config{
      Region: aws.String("ap-northeast-2"),
    },
    SharedConfigState: session.SharedConfigEnable,
  })

  if err != nil {
    println(err.Error())
    return
  }

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
    return
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

      instances[name] = *instance.InstanceId
    }
  }

  instanceId := askInstanceId(instances)

  startSession(*options.Profile, instanceId)
}
