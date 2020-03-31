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

  "github.com/spf13/cobra"
  "github.com/AlecAivazis/survey/v2"
)

type target struct {
  logPath string
  instanceIds []string
}

type streamlogsInput struct {
  profile, instanceId, logPath string
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
    selectedTarget := selectTarget()

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
      go streamlogs(ctx, &wg, streamlogsInput{
        profile: awsOpts.profile,
        instanceId: instanceId,
        logPath: selectedTarget.logPath,
        wrapColor: getColorFunc(i),
      })
    }

    wg.Wait()

    println("finished gracefully")
  },
}

func selectTarget() target {
  sess, err := newSession()

  if err != nil {
    panic(err)
  }

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

func streamlogs(ctx context.Context, wg *sync.WaitGroup, input streamlogsInput) {
  instanceId := input.instanceId

  proxyCommand := fmt.Sprintf("ProxyCommand=sh -c 'aws ssm start-session --target %%h --document-name AWS-StartSSHSession --parameters portNumber=%%p --profile %s'", input.profile)
  sshCmd := fmt.Sprintf("ec2-user@%s", instanceId)
  cmd := exec.Command("ssh", "-o", proxyCommand, "-o", "StrictHostKeyChecking=no", sshCmd, "tail", "-f", input.logPath)

  cmd.Stdin = os.Stdin
  stdout, err := cmd.StdoutPipe()

  if err != nil {
    panic(err)
  }

  scanner := bufio.NewScanner(stdout)

  if err := cmd.Start(); err != nil {
    panic(err)
  }

  for scanner.Scan() {
    fmt.Printf("[%s] %s\r\n", input.wrapColor(instanceId), scanner.Text())
  }

  if err := scanner.Err(); err != nil {
    fmt.Fprintln(os.Stderr, "reading standard output:", err)
  }

  // interrupted
  <-ctx.Done()

  if err := cmd.Process.Kill(); err != nil {
    fmt.Fprintln(os.Stderr, "failed to kill the process:", err)
  }

  if err := cmd.Wait(); err != nil {
    fmt.Fprintln(os.Stderr, "failed to wait command:", err)
  }

  println("terminated:", instanceId)

  wg.Done()
}
