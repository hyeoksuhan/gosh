package cmd

import (
  "os"
  "os/exec"
  "bufio"
  "fmt"
  "sync"

  "github.com/spf13/cobra"
  "github.com/AlecAivazis/survey/v2"
)

type target struct {
  logPath string
  instanceIds []string
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
    profile := awsOpts.profile
    selectedTarget := selectTarget()

    wg := sync.WaitGroup{}

    for _, instanceId := range selectedTarget.instanceIds {
      wg.Add(1)
      go streamlogs(&wg, profile, instanceId, selectedTarget.logPath)
    }

    wg.Wait()

    println("DONE")
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
    Message: "Eivironment name:",
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

func streamlogs(wg *sync.WaitGroup, profile, instanceId, logPath string) {
  proxyCommand := fmt.Sprintf("ProxyCommand=sh -c 'aws ssm start-session --target %%h --document-name AWS-StartSSHSession --parameters portNumber=%%p --profile %s'", profile)
  sshCmd := fmt.Sprintf("ec2-user@%s", instanceId)
  cmd := exec.Command("ssh", "-o", proxyCommand, "-o", "StrictHostKeyChecking=no", sshCmd, "tail", "-f", logPath)

  cmd.Stdin = os.Stdin
  stdout, err := cmd.StdoutPipe()

  if err != nil {
    panic(err)
  }

  scanner := bufio.NewScanner(stdout)

  go func(){
    err = cmd.Run()

    if err != nil {
      panic(err)
    }
  }()

  for scanner.Scan() {
    fmt.Printf("[%s] %s\r\n", instanceId, scanner.Text())
  }

  if err := scanner.Err(); err != nil {
    fmt.Fprintln(os.Stderr, "reading standard output:", err)
  }

  wg.Done()
}

