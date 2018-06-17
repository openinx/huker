package dashboard

import (
	"fmt"
	"github.com/qiniu/log"
	"golang.org/x/crypto/ssh"
	"strconv"
	"strings"
)

type DeployRequest struct {
	SSHUser           string `json:"sshUser"`
	SSHPrivateKey     string `json:"sshPrivateKey"`
	SSHPassword       string `json:"sshPassword"`
	HukerAgentRootDir string `json:"hukerAgentRootDir"`
	Host              string `json:"host"`
}

type hostInfo struct {
	hostname  string
	sshPort   int
	agentPort int
}

func isEmptyInput(input string) bool {
	return len(strings.Trim(input, " ")) == 0
}

func parseAgentPort(str string) (int, error) {
	if len(strings.Trim(str, " ")) == 0 {
		return 9001, nil
	}
	splits := strings.Split(str, "agentPort=")
	if len(splits) != 2 {
		return 0, fmt.Errorf("Invalid agent port format, should be like: agentPort=9001, not %s", str)
	}
	if val, err := strconv.Atoi(splits[1]); err != nil {
		return 0, fmt.Errorf("Agent port is not int type, %v", err)
	} else {
		return val, nil
	}
}

func parseSSHHostAndPort(hostAndPort string) (string, int, error) {
	splits := strings.Split(hostAndPort, ":")
	if len(splits) < 2 {
		return hostAndPort, 22, nil
	} else if len(splits) > 2 {
		return "", 0, fmt.Errorf("Invalid host format")
	} else {
		if val, err := strconv.Atoi(splits[1]); err != nil {
			return "", 0, err
		} else {
			return splits[0], val, nil
		}
	}
}

func parseHostInfo(host string) (*hostInfo, error) {
	if isEmptyInput(host) {
		return nil, fmt.Errorf("Host shouldn't be empty")
	}
	idx := strings.LastIndex(host, "/")
	if idx < 0 {
		hostname, sshPort, err := parseSSHHostAndPort(host)
		if err != nil {
			return nil, err
		}
		return &hostInfo{hostname: hostname, sshPort: sshPort, agentPort: 9001}, nil
	} else {
		hostname, sshPort, err := parseSSHHostAndPort(host[:idx])
		if err != nil {
			return nil, err
		}
		agentPort, err := parseAgentPort(host[idx:])
		if err != nil {
			return nil, err
		}
		return &hostInfo{hostname: hostname, sshPort: sshPort, agentPort: agentPort}, nil
	}
}

func deployHukerAgent(req *DeployRequest) error {
	if isEmptyInput(req.SSHUser) {
		return fmt.Errorf("SSH user is null")
	}
	if isEmptyInput(req.SSHPrivateKey) && isEmptyInput(req.SSHPassword) {
		return fmt.Errorf("Both SSH private key and SSH password are null, should use key auth or pass auth.")
	}
	if isEmptyInput(req.HukerAgentRootDir) {
		return fmt.Errorf("Huker agent root dir is empty.")
	}
	h, err := parseHostInfo(req.Host)
	if err != nil {
		return err
	}

	var authMethod ssh.AuthMethod
	if isEmptyInput(req.SSHPrivateKey) {
		authMethod = ssh.Password(req.SSHPassword)
	} else {
		sig, err := ssh.ParsePrivateKey([]byte(req.SSHPrivateKey))
		if err != nil {
			log.Errorf("Parse private key failed: %v", err)
			return err
		}
		authMethod = ssh.PublicKeys(sig)
	}

	sshConfig := &ssh.ClientConfig{
		User: req.SSHUser,
		Auth: []ssh.AuthMethod{authMethod},
	}

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", h.hostname, h.sshPort), sshConfig)
	if err != nil {
		log.Error("Failed to dial: %v", err)
		return fmt.Errorf("Failed to dial: %v", err)
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		log.Error("Failed to create new session to sshd-server, %v", err)
		return err
	}
	defer session.Close()
	return session.Run("ls -al")
}
