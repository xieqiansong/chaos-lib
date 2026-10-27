package portfwd

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"chaos-go/internal/framework/crud"
)

var ErrDBUnavailable = errors.New("portfwd: database unavailable")

var (
	ErrRuleNotFound = errors.New("portfwd: 端口转发不存在")
	ErrConnNotFound = errors.New("portfwd: SSH 连接不存在")
)

const (
	DirectionLocal = "local"

	DirectionRemote = "remote"

	DirectionDirect = "direct"

	DefaultLocalBindAddress = "0.0.0.0"

	DefaultRemoteBindAddress = "127.0.0.1"

	DefaultDirectBindAddress = "0.0.0.0"
)

const (
	AuthTypePassword = "password"
	AuthTypeKey      = "key"
)

func isLocalSideListen(direction string) bool {
	return direction == DirectionLocal || direction == DirectionDirect
}

type PortForwarding struct {
	crud.BaseModel
	Name            string
	Direction       string
	Port            int
	BindAddress     string
	TargetHost      string
	TargetPort      int
	SshConnectionId int
	Status          bool
	LastError       string
	Remark          string
}

func (PortForwarding) TableName() string {
	return "port_forwarding"
}

func (pf *PortForwarding) displayBindAddress() string {
	if strings.TrimSpace(pf.BindAddress) != "" {
		return pf.BindAddress
	}
	switch pf.Direction {
	case DirectionRemote:
		return DefaultRemoteBindAddress
	case DirectionDirect:
		return DefaultDirectBindAddress
	default:
		return DefaultLocalBindAddress
	}
}

func (pf *PortForwarding) normalize() error {
	switch pf.Direction {
	case "":
		pf.Direction = DirectionLocal
	case DirectionLocal, DirectionRemote, DirectionDirect:
	default:
		return fmt.Errorf("转发方向不合法，只能为 %s、%s 或 %s", DirectionLocal, DirectionRemote, DirectionDirect)
	}
	if strings.TrimSpace(pf.BindAddress) == "" {
		switch pf.Direction {
		case DirectionRemote:
			pf.BindAddress = DefaultRemoteBindAddress
		case DirectionDirect:
			pf.BindAddress = DefaultDirectBindAddress
		default:
			pf.BindAddress = DefaultLocalBindAddress
		}
	}
	pf.BindAddress = strings.TrimSpace(pf.BindAddress)
	return nil
}

func (pf *PortForwarding) listenAddr() string {
	return net.JoinHostPort(pf.BindAddress, strconv.Itoa(pf.Port))
}

func (pf *PortForwarding) targetAddr() string {
	return fmt.Sprintf("%s:%d", pf.TargetHost, pf.TargetPort)
}

func (pf *PortForwarding) directionLabel() string {
	switch pf.Direction {
	case DirectionRemote:
		return "远端"
	case DirectionDirect:
		return "直接"
	default:
		return "本地"
	}
}

func (pf *PortForwarding) validateFields() error {
	if err := validatePort(pf.Port); err != nil {
		return fmt.Errorf("%s监听端口不合法: %v", pf.directionLabel(), err)
	}
	if strings.ContainsAny(pf.BindAddress, " \t/") {
		return fmt.Errorf("监听地址不合法: %s", pf.BindAddress)
	}
	if strings.TrimSpace(pf.TargetHost) == "" {
		return fmt.Errorf("目标主机不能为空")
	}
	if err := validatePort(pf.TargetPort); err != nil {
		return fmt.Errorf("目标端口不合法: %v", err)
	}
	return nil
}

type SshConnection struct {
	crud.BaseModel
	Name       string
	Host       string
	Port       int
	Username   string
	AuthType   string
	Password   string
	PrivateKey string
	Passphrase string
	Remark     string
}

func (SshConnection) TableName() string {
	return "ssh_connections"
}

func (conn *SshConnection) normalizedPort() int {
	if conn.Port <= 0 {
		return 22
	}
	return conn.Port
}

func (conn *SshConnection) sshAddr() string {
	return net.JoinHostPort(conn.Host, strconv.Itoa(conn.normalizedPort()))
}

func (conn *SshConnection) normalizeAuthType() {
	if conn.AuthType != AuthTypeKey {
		conn.AuthType = AuthTypePassword
	}
}

func validatePort(port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("端口号必须在1-65535之间")
	}
	return nil
}

func (conn *SshConnection) validateForSave(needCredential bool) error {
	if strings.TrimSpace(conn.Name) == "" {
		return fmt.Errorf("名称不能为空")
	}
	if strings.TrimSpace(conn.Host) == "" {
		return fmt.Errorf("SSH 主机不能为空")
	}
	if strings.TrimSpace(conn.Username) == "" {
		return fmt.Errorf("SSH 用户名不能为空")
	}
	if err := validatePort(conn.normalizedPort()); err != nil {
		return err
	}

	conn.Port = conn.normalizedPort()
	conn.normalizeAuthType()
	if !needCredential {
		return nil
	}
	if conn.AuthType == AuthTypeKey {
		if strings.TrimSpace(conn.PrivateKey) == "" {
			return fmt.Errorf("私钥认证需要提供私钥")
		}
		return nil
	}
	if conn.Password == "" {
		return fmt.Errorf("密码认证需要提供密码")
	}
	return nil
}
