package cmd

import (
  "os"
  "os/exec"
  "bufio"
  "fmt"
  "sync"
  "context"
  "os/signal"
  "syscall"
  "encoding/json"

  "github.com/spf13/cobra"
  "github.com/AlecAivazis/survey/v2"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/ssm"
)

type target struct {
  logPath string
  instanceIds []string
}

type streamlogsInput struct {
  awsOpts AWSopts
  session *session.Session
  instanceId string
  logPath string
  wrapColor func(...interface {}) string
}

func init() {
  RootCmd.AddCommand(cmdEBLogs)
}

var cmdEBLogs = &cobra.Command{
  Use: "eblogs",
  Short: "tail -f Elastic Beanstalk logs",
  Long: "Tail forwarding Elastic Beanstalk logs for its platform",
  PreRun: func(cmd *cobra.Command, args []string) {
    handleSurveyError(setProfile())
  },
  Run: func(cmd *cobra.Command, args []string) {
    sess, err := newSession()
    if err != nil {
      panic(err)
    }

    selectedTarget := selectTarget(sess)

    sigs := make(chan os.Signal)
    signal.Notify(sigs, syscall.SIGINT)

    ctx, cancel := context.WithCancel(context.Background())

    go func() {
      sig := <-sigs
      fmt.Println(sig, ", terminate process")
      cancel()
    }()

    wg := sync.WaitGroup{}

    for i, instanceId := range selectedTarget.instanceIds {
      wg.Add(1)

      input := streamlogsInput{
        awsOpts: awsOpts,
        session: sess,
        instanceId: instanceId,
        logPath: selectedTarget.logPath,
        wrapColor: getColorFunc(i),
      }

      go func(input streamlogsInput) {
        sessionId, err := streamlogs(ctx, &wg, input)

        if err != nil {
          if err := terminateSession(ctx, sess, sessionId); err != nil {
            panic(err)
          }
        }
      }(input)
    }

    wg.Wait()

    println("finished gracefully")
  },
}

func selectTarget(sess *session.Session) target {
  ebService, err := newEbService(sess)

  if err != nil {
    panic(err)
  }

  selectedEnvName, err := selectEnv(ebService.envNames)

  if err != nil {
    panic(err)
  }

  instanceIds, err := ebService.getInstanceIds(selectedEnvName)

  if err != nil {
    panic(err)
  }

  selectedIds, err := selectInstanceIds(instanceIds)

  if err != nil {
    panic(err)
  }

  return target{
    ebService.getLogPath(selectedEnvName),
    selectedIds,
  }
}

func selectEnv(envs []string) (selectedEnv string, err error) {
  err = survey.AskOne(&survey.Select{
    Message: "Environment name:",
    Options: envs,
    VimMode: true,
  }, &selectedEnv)

  return
}

func selectInstanceIds(instanceIds []string) (selectedIds []string, err error) {
  err = survey.AskOne(&survey.MultiSelect{
    Message: "Select instances:",
    Options: instanceIds,
  }, &selectedIds)

  return
}

func streamlogs(ctx context.Context, wg *sync.WaitGroup, input streamlogsInput) (sessionId string, err error) {
  region := awsOpts.region
  profile := awsOpts.profile
  docName := "AWS-StartSSHSession"
  port := "22"
  instanceId := input.instanceId

  sessionInput := &ssm.StartSessionInput{
    DocumentName: &docName,
    Parameters:   map[string][]*string{"portNumber": []*string{&port}},
    Target:       &instanceId,
  }

  sessOutput, endpoint, err := createSession(input.session, sessionInput)
  if err != nil {
    return
  }

  outputJson, err := json.Marshal(sessOutput)
  if err != nil {
    return
  }

  sessionInputJson, err := json.Marshal(sessionInput)
  if err != nil {
    return
  }

  sessionId = *sessOutput.SessionId

  proxyCommand := fmt.Sprintf("ProxyCommand=session-manager-plugin '%s' %s %s %s '%s' %s",
    string(outputJson), region, "StartSession", profile, string(sessionInputJson), endpoint)

  sshArgs := []string{
    "-o",
    proxyCommand,
    "-o",
    "StrictHostKeyChecking=no",
    fmt.Sprintf("ec2-user@%s", instanceId),
  }

  sshCommand := []string{
    "tail",
    "-f",
    input.logPath,
  }

  args := append(sshArgs, sshCommand...)

  cmd := exec.Command("ssh", args...)
  cmd.Stdin = os.Stdin
  stdout, err := cmd.StdoutPipe()

  if err != nil {
    return
  }

  scanner := bufio.NewScanner(stdout)

  err = cmd.Start()
  if err != nil {
    return
  }

  for scanner.Scan() {
    fmt.Printf("[%s] %s\r\n", input.wrapColor(instanceId), scanner.Text())
  }

  err = scanner.Err()
  if err != nil {
    fmt.Fprintln(os.Stderr, "reading standard output:", err)
    return
  }

  // interrupted
  <-ctx.Done()

  err = cmd.Process.Kill()
  if err != nil {
    fmt.Fprintln(os.Stderr, "failed to kill the process:", err)
    return
  }

  err = cmd.Wait()
  if err != nil {
    fmt.Fprintln(os.Stderr, "failed to wait command:", err)
    return
  }

  println("terminated:", instanceId)

  wg.Done()

  return
}
