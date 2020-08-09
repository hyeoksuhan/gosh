package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
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
	logPath string
	ids     []string
}

type streamlogsInput struct {
	service    gossm.SSMservice
	instanceID string
	logPath    string
	grep       string
	colorf     func(...interface{}) string
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
		grep := askGrep()

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
				grep:       grep,
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

func askGrep() string {
	grep := ""

	survey.AskOne(&survey.Input{
		Message: "grep",
	}, &grep)

	return grep
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
		line := strings.Replace(scanner.Text(), input.grep, input.colorf(input.grep), 1)
		if input.grep != "" && !strings.Contains(line, input.grep) {
			continue
		}

		fmt.Printf("[%s] %s\r\n", input.colorf(target), line)
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

func terminateSession(svc gossm.SSMservice, sid string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return svc.TerminateSession(ctx, sid)
}
