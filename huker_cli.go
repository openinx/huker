package huker

import (
	"flag"
	"net/http"
)

type DeployService interface {
	bootstrap(job string) error
	start(job string, flags ...flag.Flag) error
	rolling_update(job string, flags ...flag.Flag) error
	restart(job string, flags ...flag.Flag) error
}

type HDFSService struct {
}

func (s *HDFSService) bootstrap(job string) error {
	return nil
}

func (s *HDFSService) start(job string, flags ...flag.Flag) error {
	return nil
}

func (s *HDFSService) rolling_update(job string, flags ...flag.Flag) error {
	return nil
}

func (s *HDFSService) restart(job string, flags ...flag.Flag) error {
	return nil
}

type ZookeeperService struct {
}

func (s *ZookeeperService) bootstrap(job string) error {
	http.Get("")

	return nil
}

func (s *ZookeeperService) start(job string, flags ...flag.Flag) error {
	return nil
}

func (s *ZookeeperService) rolling_update(job string, flags ...flag.Flag) error {
	return nil
}

func (s *ZookeeperService) restart(job string, flags ...flag.Flag) error {
	return nil
}
