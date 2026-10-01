package portfwd

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// 本文件是 SSH 建连的基础设施实现（凭据解析 + 拨号），只被 service 调用。
// 任何错误信息都不包含私钥或口令内容。

// authMethods 按认证方式构造 SSH 认证方法。
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

// dial 建立 SSH 连接。
// Host Key 采用 InsecureIgnoreHostKey：本工具是自托管单用户场景，不校验服务器指纹以换取接入便利，
// 由「测试连接」接口回传服务器标识供人工比对；后续如需严格校验再引入 known_hosts。
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
