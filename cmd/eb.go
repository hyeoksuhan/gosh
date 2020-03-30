package cmd

import (
  "strings"

  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  eb "github.com/aws/aws-sdk-go/service/elasticbeanstalk"
)

type ebService struct {
  ebInstance *eb.ElasticBeanstalk
  envNames []string
  envStackMap map[string]string
  envPathMap map[string]string
}

func newEbService(sess *session.Session) (instance ebService, err error ) {
  svc := eb.New(sess)

  envs, err := svc.DescribeEnvironments(&eb.DescribeEnvironmentsInput{})

  if err != nil {
    return
  }

  instance = ebService{
    ebInstance: svc,
    envNames: []string{},
    envStackMap: make(map[string]string),
    envPathMap: map[string]string{
      "node.js": "/var/log/nodejs/nodejs.log",
      "java": "/var/log/web-1.log",
    },
  }

  for _, env := range(envs.Environments) {
    envName := *env.EnvironmentName
    stackName := *env.SolutionStackName

    instance.envNames = append(instance.envNames, *env.EnvironmentName)
    instance.envStackMap[envName] = stackName
  }

  return
}

func (svc ebService) isValidEnvName(envName string) bool {
  for _, v := range svc.envNames {
    if v == envName {
      return true
    }
  }

  return false
}


func (svc ebService) getInstanceIds(envName string) (instanceIds []string, err error) {
  validEnvName := svc.isValidEnvName(envName)

  if !validEnvName {
    return
  }

  envResources, err := svc.ebInstance.DescribeEnvironmentResources(&eb.DescribeEnvironmentResourcesInput{
    EnvironmentName: aws.String(envName),
  })

  if err != nil {
    return
  }

  for _, instance := range(envResources.EnvironmentResources.Instances) {
    instanceIds = append(instanceIds, *instance.Id)
  }

  return
}

func (svc ebService) getStackName(envName string) string {
  return svc.envStackMap[envName]
}

func (svc ebService) getLogPath(envName string) string {
  stackName := svc.getStackName(envName)

  for k, v := range svc.envPathMap {
    if strings.Contains(strings.ToLower(stackName), k) {
      return v
    }
  }

  return ""
}

