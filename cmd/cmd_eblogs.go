package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/hyeoksuhan/gosh/service/goeb"
	"github.com/hyeoksuhan/gosh/service/gossm"
	"github.com/spf13/cobra"
)

type target struct {
	logPath []string
	ids     []string
}

type streamlogsInput struct {
	service    gossm.SSMservice
	instanceID string
	logPath    []string
	grepOpt    grepOpt
	colorf     func(...interface{}) string
}

type grepOpt struct {
	regexp string
	aCount int
}

func (o grepOpt) isEmpty() bool {
	return strings.TrimSpace(o.regexp) == ""
}

func init() {
	rootCmd.AddCommand(cmdEBLogs)
}

var cmdEBLogs = &cobra.Command{
	Use:   "eblogs",
	Short: "tail -f Elastic Beanstalk logs",
	Long:  "Tail forwarding Elastic Beanstalk logs for its platform",
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

		target := selectTarget(svc.Session)
		grepOpt := askGrepOpt()

		ch := make(chan os.Signal)
		signal.Notify(ch, syscall.SIGINT)

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			fmt.Println(<-ch, ", terminate process")
			cancel()
		}()

		wg := sync.WaitGroup{}

		for i, id := range target.ids {
			wg.Add(1)

			input := streamlogsInput{
				service:    svc,
				instanceID: id,
				logPath:    target.logPath,
				colorf:     colorf(i),
				grepOpt:    grepOpt,
			}

			go func(input streamlogsInput) {
				sid, err := streamlogs(ctx, &wg, input)

				defer func(sid string) {
					if sid == "" {
						return
					}

					if err := terminateSession(svc, sid); err != nil {
						fatal(err)
					}
				}(sid)

				if err != nil {
					panic(err)
				}
			}(input)
		}

		wg.Wait()

		println("finished gracefully")
	},
}

func selectTarget(sess *session.Session) target {
	svc, err := goeb.New(sess)
	if err != nil {
		fatal(err)
	}

	envName, err := selectEnv(svc.EnvNames)
	if err != nil {
		fatal(err)
	}

	srcids, err := svc.GetInstanceIds(envName)
	if err != nil {
		fatal(err)
	}

	ids, err := selectInstanceIds(srcids)
	if err != nil {
		fatal(err)
	}

	return target{
		svc.GetLogPath(envName),
		ids,
	}
}

func selectEnv(envs []string) (env string, err error) {
	err = survey.AskOne(&survey.Select{
		Message: "Environment name:",
		Options: envs,
		VimMode: true,
	}, &env)

	return
}

func askGrepOpt() grepOpt {
	grepOpt := grepOpt{}

	survey.AskOne(&survey.Input{
		Message: "grep regex:",
	}, &grepOpt.regexp)

	if !grepOpt.isEmpty() {
		survey.AskOne(&survey.Input{
			Message: "grep -A:",
		}, &grepOpt.aCount)
	}

	return grepOpt
}

func selectInstanceIds(ids []string) (selectedIds []string, err error) {
	err = survey.AskOne(&survey.MultiSelect{
		Message: "Select instances:",
		Options: ids,
	}, &selectedIds)

	return
}

func streamlogs(ctx context.Context, wg *sync.WaitGroup, input streamlogsInput) (sid string, err error) {
	docName := "AWS-StartSSHSession"
	port := "22"
	target := input.instanceID

	output, err := input.service.StartSession(&ssm.StartSessionInput{
		DocumentName: &docName,
		Parameters:   map[string][]*string{"portNumber": {&port}},
		Target:       &target,
	})
	if err != nil {
		return
	}

	sid = output.SessionID

	cmdWithArgs := func() []interface{} {
		l := append([]string{output.Command}, output.CommandArgs...)
		r := make([]interface{}, len(l))
		for i, v := range l {
			r[i] = v
		}
		return r
	}()

	proxy := fmt.Sprintf("ProxyCommand=%s '%s' %s %s %s '%s' %s", cmdWithArgs...)

	sshArgs := []string{
		"-o",
		proxy,
		"-o",
		"StrictHostKeyChecking=no",
		fmt.Sprintf("ec2-user@%s", target),
	}

	sshCommand := []string{
		"tail",
	}

	for _, p := range input.logPath {
		sshCommand = append(sshCommand, "-F", p)
	}

	args := append(sshArgs, sshCommand...)

	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	stdout, err := cmd.StdoutPipe()

	if err != nil {
		return
	}

	scanner := bufio.NewScanner(stdout)
	// to prevent 'token too long' error
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	err = cmd.Start()
	if err != nil {
		return
	}

	grepOpt := input.grepOpt
	re := regexp.MustCompile(grepOpt.regexp)
	cTarget := input.colorf(target)
	remain := 0
	for scanner.Scan() {
		line := scanner.Text()

		if grepOpt.isEmpty() {
			printTargetLog(cTarget, line)
			continue
		}

		match := re.FindAllString(line, -1)
		matched := len(match) > 0
		loggable := matched || remain > 0

		if !loggable {
			continue
		}

		for _, m := range match {
			line = strings.Replace(line, m, input.colorf(m), 1)
		}

		printTargetLog(cTarget, line)

		if grepOpt.aCount == 0 {
			continue
		}

		if matched {
			remain = grepOpt.aCount
			continue
		}

		if remain--; remain == 0 {
			fmt.Printf(input.colorf("--") + "\r\n")
		}
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

	println("terminated:", target)

	wg.Done()

	return
}

func printTargetLog(target string, log string) {
	fmt.Printf("[%s] %s\r\n", target, log)
}

func terminateSession(svc gossm.SSMservice, sid string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return svc.TerminateSession(ctx, sid)
}
