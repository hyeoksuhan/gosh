package gossm

import (
  "context"
  "encoding/json"

  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/ssm"
)

const plugin = "session-manager-plugin"

type SSMservice struct {
  Profile, Region string
  Session *session.Session
}

type SSMstartOutput struct {
  Command string
  CommandArgs []string
  SessionId string
}

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
    Region: region,
    Session: sess,
  }

  return
}

func (s SSMservice) StartSession(input *ssm.StartSessionInput) (SSMstartOutput, error) {
  result := SSMstartOutput{}

  svc := ssm.New(s.Session)

  output, err := svc.StartSession(input)
  if err != nil {
    return result, err
  }

  outputJson, err := json.Marshal(output)
  if err != nil {
    return result, err
  }

  inputJson, err := json.Marshal(input)
  if err != nil {
    return result, err
  }

  result.Command = plugin
  result.CommandArgs = []string{
    string(outputJson),
    s.Region,
    "StartSession",
    s.Profile,
    string(inputJson),
    svc.Endpoint,
  }
  result.SessionId = *output.SessionId

  return result, nil
}

func (s SSMservice) TerminateSession(ctx context.Context, sessionId string) error {
  svc := ssm.New(s.Session)

  _, err := svc.TerminateSessionWithContext(ctx, &ssm.TerminateSessionInput{
    SessionId: &sessionId,
  })

  return err
}

