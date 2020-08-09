package gossm

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

const plugin = "session-manager-plugin"

// SSMservice is service for AWS SSM
type SSMservice struct {
	Profile, Region string
	Session         *session.Session
}

// SSMstartOutput is result for StartSession
type SSMstartOutput struct {
	Command     string
	CommandArgs []string
	SessionID   string
}

// New creates new SSMservice
func New(region, profile string) (svc SSMservice, err error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: profile,
		Config: aws.Config{
			Region: aws.String(region),
		},
		SharedConfigState: session.SharedConfigEnable,
	})

	if err != nil {
		return
	}

	svc = SSMservice{
		Profile: profile,
		Region:  region,
		Session: sess,
	}

	return
}

// StartSession start ssm session
func (s SSMservice) StartSession(input *ssm.StartSessionInput) (SSMstartOutput, error) {
	result := SSMstartOutput{}

	svc := ssm.New(s.Session)

	output, err := svc.StartSession(input)
	if err != nil {
		return result, err
	}

	outputJSON, err := json.Marshal(output)
	if err != nil {
		return result, err
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return result, err
	}

	result.Command = plugin
	result.CommandArgs = []string{
		string(outputJSON),
		s.Region,
		"StartSession",
		s.Profile,
		string(inputJSON),
		svc.Endpoint,
	}
	result.SessionID = *output.SessionId

	return result, nil
}

// TerminateSession terminate ssm session
func (s SSMservice) TerminateSession(ctx context.Context, sessionID string) error {
	svc := ssm.New(s.Session)

	_, err := svc.TerminateSessionWithContext(ctx, &ssm.TerminateSessionInput{
		SessionId: &sessionID,
	})

	return err
}
