package cmd

import (
  "os"
  "os/exec"
  "bufio"
  "fmt"
  "sync"
  "strings"

  "github.com/spf13/cobra"
  "github.com/AlecAivazis/survey/v2"
  "github.com/aws/aws-sdk-go/aws"
  eb "github.com/aws/aws-sdk-go/service/elasticbeanstalk"
)

type ebLogPath struct {
  pathTable map[string]string
}

func (e ebLogPath) getPath(stackName string) string {
  for k, v := range e.pathTable {
    if strings.Contains(strings.ToLower(stackName), k) {
      return v
    }
  }

  return ""
}

func newEbLogPath() ebLogPath {
  table := map[string]string{
    "node.js": "/var/log/nodejs/nodejs.log",
    "java": "/var/log/web-1.log",
  }

  return ebLogPath{table}
}

func init() {
  RootCmd.AddCommand(cmdEBLogs)
}

type logTarget struct {
  path string
  instanceIds []string
}

var cmdEBLogs = &cobra.Command{
  Use: "eblogs",
  Short: "tail -f elastic beanstalk logs",
  Long: "Tail forwarding Elastic Beanstalk logs for its platform",
  PreRun: func(cmd *cobra.Command, args []string) {
    setProfile()
  },
  Run: func(cmd *cobra.Command, args []string) {
    logTarget := getTarget()

    wg := sync.WaitGroup{}

    for _, target := range logTarget.instanceIds {
      wg.Add(1)
      go sshlog(&wg, target, logTarget.path)
    }

    wg.Wait()

    println("DONE")
  },
}

func getTarget() logTarget {
  sess, err := newSession()

  if err != nil {
    println(err.Error())
    panic(err)
  }

  svc := eb.New(sess)

  envs, err := svc.DescribeEnvironments(&eb.DescribeEnvironmentsInput{})

  if err != nil {
    panic(err)
  }

  var envNames []string
  envStacks := make(map[string]string)

  for _, env := range(envs.Environments) {
    envName := *env.EnvironmentName
    stackName := *env.SolutionStackName

    envNames = append(envNames, *env.EnvironmentName)
    envStacks[envName] = stackName
  }

  selectedName := ""

  survey.AskOne(&survey.Select{
    Message: "Eivironment name:",
    Options: envNames,
    VimMode: true,
  }, &selectedName)


  envResources, err := svc.DescribeEnvironmentResources(&eb.DescribeEnvironmentResourcesInput{
    EnvironmentName: aws.String(selectedName),
  })

  if err != nil {
    panic(err)
  }

  instanceIds := []string{}

  for _, instance := range(envResources.EnvironmentResources.Instances) {
    instanceIds = append(instanceIds, *instance.Id)
  }

  selectedIds := []string{}

  survey.AskOne(&survey.MultiSelect{
    Message: "Select instances:",
    Options: instanceIds,
  }, &selectedIds)

  logPath := newEbLogPath()
  eblogpath := logPath.getPath(envStacks[selectedName])

  return logTarget{
    eblogpath,
    selectedIds,
  }
}

func sshlog(wg *sync.WaitGroup, instanceId, logpath string) {
  profile := awsOpts.profile

  proxyCommand := fmt.Sprintf("ProxyCommand=sh -c 'aws ssm start-session --target %%h --document-name AWS-StartSSHSession --parameters portNumber=%%p --profile %s'", profile)
  extCmd := fmt.Sprintf("ec2-user@%s", instanceId)
  cmd := exec.Command("ssh", "-o", proxyCommand, "-o", "StrictHostKeyChecking=no", extCmd, "tail", "-f", logpath)

  cmd.Stdin = os.Stdin
  stdout, err := cmd.StdoutPipe()

  if err != nil {
    fmt.Println("@@error:", err)
  }

  scanner := bufio.NewScanner(stdout)

  go func(){
    err = cmd.Run()
    if err != nil {
      fmt.Println("##error:", err)
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

