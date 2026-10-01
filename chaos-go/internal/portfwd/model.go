package portfwd

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"chaos-go/internal/crud"
	"golang.org/x/crypto/ssh"
)

// ErrDBUnavailable 表示数据库单例不可用（极端情况下）。
var ErrDBUnavailable = errors.New("portfwd: database unavailable")

const (
	// DirectionLocal 本地转发（等价 ssh -L）：本机监听 → SSH 服务器侧解析目标
	DirectionLocal = "local"
	// DirectionRemote 远程转发（等价 ssh -R）：SSH 服务器侧监听 → 本机侧解析目标
	DirectionRemote = "remote"
	// DirectionDirect 直接转发：本机监听 → 直接（纯 TCP）拨向目标，不经 SSH 隧道
	DirectionDirect = "direct"

	// DefaultLocalBindAddress 本地转发缺省监听地址（监听全部网卡，与既有行为一致）
	DefaultLocalBindAddress = "0.0.0.0"
	// DefaultRemoteBindAddress 远程转发缺省监听地址（与 OpenSSH ssh -R 不带 bind_address 一致）
	DefaultRemoteBindAddress = "127.0.0.1"
	// DefaultDirectBindAddress 直接转发缺省监听地址（监听全部网卡）
	DefaultDirectBindAddress = "0.0.0.0"
)

const (
	AuthTypePassword = "password"
	AuthTypeKey      = "key"
)

// isLocalSideListen 该方向是否在本机监听（本地转发与直接转发都在本机侧监听）。
func isLocalSideListen(direction string) bool {
	return direction == DirectionLocal || direction == DirectionDirect
}

// ── PortForwarding ─────────────────────────────────────────────────

// PortForwarding 端口转发规则。
// 嵌入 crud.BaseModel 以获得统一主键 / 时间戳 / 软删除。
// Direction 决定「监听」发生在哪一侧：local 在本机监听，remote 在 SSH 服务器侧监听。
// Status 为「期望运行」标记（供重启恢复 / 自愈使用）；实际运行状态以内存为准（见 toResponse）。
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

// PortForwardingResponse 对外 DTO；Status 以内存中实际运行状态为准。
type PortForwardingResponse struct {
	ID              int    `json:"ID"`
	Name            string `json:"Name"`
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

// displayBindAddress 监听地址展示值：空值时按方向取默认（仅用于展示，运行期 normalize 另行兜底）。
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

// normalize 补齐方向与监听地址：方向空值视为 local（兼容本次变更前的存量数据）。
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

// listenAddr 监听地址：local 为本机监听地址，remote 为 SSH 服务器侧监听地址。
func (pf *PortForwarding) listenAddr() string {
	return net.JoinHostPort(pf.BindAddress, strconv.Itoa(pf.Port))
}

// targetAddr 目标地址：local 由 SSH 服务器侧解析，remote 由本机侧解析。
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

func (pf *PortForwarding) toResponse() PortForwardingResponse {
	running, lastErr := GlobalPortForwarder.Status(pf.ID)
	return PortForwardingResponse{
		ID:              pf.ID,
		Name:            pf.Name,
		Direction:       pf.Direction,
		Port:            pf.Port,
		BindAddress:     pf.displayBindAddress(),
		TargetHost:      pf.TargetHost,
		TargetPort:      pf.TargetPort,
		SshConnectionId: pf.SshConnectionId,
		Status:          running,
		LastError:       lastErr,
		Remark:          pf.Remark,
	}
}

// portForwardingsToResponse 整批把 []*PortForwarding 转成响应 DTO（供 crud.ToResponse 调用）。
func portForwardingsToResponse(rows []*PortForwarding) any {
	out := make([]PortForwardingResponse, 0, len(rows))
	for _, r := range rows {
		// 存量行可能没有 direction，读路径同样按 local 兜底
		_ = r.normalize()
		out = append(out, r.toResponse())
	}
	return out
}

// validate 校验规则字段；需关联已存在的 SSH 连接（direct 除外）。空名称自动生成。
func (pf *PortForwarding) validate() error {
	if err := pf.normalize(); err != nil {
		return err
	}
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
	// 直接转发不经 SSH 隧道，无需关联 SSH 连接；其余方向必须关联一条已存在的连接。
	if pf.Direction != DirectionDirect {
		if pf.SshConnectionId <= 0 {
			return fmt.Errorf("请选择 SSH 连接")
		}
		if exists, err := SshConnExists(pf.SshConnectionId); err != nil {
			return fmt.Errorf("校验 SSH 连接失败: %v", err)
		} else if !exists {
			return fmt.Errorf("SSH 连接不存在")
		}
	}
	if strings.TrimSpace(pf.Name) == "" {
		switch pf.Direction {
		case DirectionRemote:
			pf.Name = fmt.Sprintf("[R] %s → %s", pf.listenAddr(), pf.targetAddr())
		case DirectionDirect:
			pf.Name = fmt.Sprintf("[D] %s → %s", pf.listenAddr(), pf.targetAddr())
		default:
			pf.Name = fmt.Sprintf("[L] %s → %s", pf.listenAddr(), pf.targetAddr())
		}
	}
	return nil
}

// ── SshConnection ─────────────────────────────────────────────────

// SshConnection SSH 连接信息。
// 凭据（Password / PrivateKey / Passphrase）只用于后端建连，一律不经 API 返回、不写入日志。
// 嵌入 crud.BaseModel 以获得统一主键 / 时间戳 / 软删除。
type SshConnection struct {
	crud.BaseModel
	Name       string
	Host       string
	Port       int
	Username   string
	AuthType   string
	Password   string `json:"-"`
	PrivateKey string `json:"-"`
	Passphrase string `json:"-"`
	Remark     string
}

func (SshConnection) TableName() string {
	return "ssh_connections"
}

// SshConnectionResponse 对外 DTO：只暴露「是否已配置凭据」，绝不返回凭据明文。
type SshConnectionResponse struct {
	ID            int    `json:"ID"`
	Name          string `json:"Name"`
	Host          string
	Port          int
	Username      string
	AuthType      string
	Remark        string
	HasPassword   bool
	HasPrivateKey bool
	HasPassphrase bool
}

func (conn *SshConnection) toResponse() SshConnectionResponse {
	return SshConnectionResponse{
		ID:            conn.ID,
		Name:          conn.Name,
		Host:          conn.Host,
		Port:          conn.normalizedPort(),
		Username:      conn.Username,
		AuthType:      conn.AuthType,
		Remark:        conn.Remark,
		HasPassword:   conn.Password != "",
		HasPrivateKey: conn.PrivateKey != "",
		HasPassphrase: conn.Passphrase != "",
	}
}

// sshConnsToResponse 整批把 []*SshConnection 转成响应 DTO（供 crud.ToResponse 调用）。
func sshConnsToResponse(rows []*SshConnection) any {
	out := make([]SshConnectionResponse, 0, len(rows))
	for _, c := range rows {
		out = append(out, c.toResponse())
	}
	return out
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

// authMethods 按认证方式构造 SSH 认证方法。任何错误信息都不包含私钥或口令内容。
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

// validateForSave 校验连接的基本字段；needCredential 为 true 时要求对应凭据已配置。
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
	// 落库前把空端口补成默认 22，避免库里出现 0
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
