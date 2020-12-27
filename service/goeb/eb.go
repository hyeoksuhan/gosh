package goeb

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	eb "github.com/aws/aws-sdk-go/service/elasticbeanstalk"
)

// EBservice is service for AWS ElasticBeanstalk
type EBservice struct {
	ebInstance  *eb.ElasticBeanstalk
	EnvNames    []string
	envStackMap map[string]string
	envPathMap  map[string][]string
}

// New creates new EBservice
func New(sess *session.Session) (instance EBservice, err error) {
	svc := eb.New(sess)

	envs, err := svc.DescribeEnvironments(&eb.DescribeEnvironmentsInput{})
	if err != nil {
		return
	}

	instance = EBservice{
		ebInstance:  svc,
		EnvNames:    []string{},
		envStackMap: make(map[string]string),
		envPathMap: map[string][]string{
			"node.js": []string{"/var/log/nodejs/nodejs.log"},
			"java":    []string{"/var/log/web-1.log", "/var/log/web-1.error.log"},
		},
	}

	for _, env := range envs.Environments {
		envName := *env.EnvironmentName
		stackName := *env.SolutionStackName

		instance.EnvNames = append(instance.EnvNames, *env.EnvironmentName)
		instance.envStackMap[envName] = stackName
	}

	return
}

func (svc EBservice) isValidEnvName(envName string) bool {
	for _, v := range svc.EnvNames {
		if v == envName {
			return true
		}
	}

	return false
}

// GetInstanceIds returns EC2 instance ids
func (svc EBservice) GetInstanceIds(envName string) (instanceIds []string, err error) {
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

	for _, instance := range envResources.EnvironmentResources.Instances {
		instanceIds = append(instanceIds, *instance.Id)
	}

	return
}

func (svc EBservice) getStackName(envName string) string {
	return svc.envStackMap[envName]
}

// GetLogPath returns app log path matched with platform
func (svc EBservice) GetLogPath(envName string) []string {
	stackName := svc.getStackName(envName)

	for k, v := range svc.envPathMap {
		if strings.Contains(strings.ToLower(stackName), k) {
			return v
		}
	}

	return []string{}
}
