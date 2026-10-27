package portfwd

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

func (conn *SshConnection) authMethods() ([]ssh.AuthMethod, error) {
	switch conn.AuthType {
	case AuthTypeKey:
		if strings.TrimSpace(conn.PrivateKey) == "" {
			return nil, fmt.Errorf("未配置私钥")
		}
		var (
			signer ssh.Signer
			err    error
		)
		if conn.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(conn.PrivateKey), []byte(conn.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(conn.PrivateKey))
		}
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %v", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	default:
		if conn.Password == "" {
			return nil, fmt.Errorf("未配置密码")
		}
		return []ssh.AuthMethod{ssh.Password(conn.Password)}, nil
	}
}

func (conn *SshConnection) dial() (*ssh.Client, error) {
	auths, err := conn.authMethods()
	if err != nil {
		return nil, err
	}
	clientConfig := &ssh.ClientConfig{
		User:            conn.Username,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	return ssh.Dial("tcp", conn.sshAddr(), clientConfig)
}
