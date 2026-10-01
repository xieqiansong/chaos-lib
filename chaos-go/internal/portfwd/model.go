// Package portfwd 管理 SSH 连接与端口转发规则（local `ssh -L` / remote `ssh -R` / 直连 direct）。
//
// 分层（见 chaos-lib/AGENTS.md「分层契约」）：
//   - model.go：实体 + 常量 + 纯领域逻辑（规范化、地址推导、不依赖 IO 的字段校验）
//   - repository.go：数据访问
//   - client.go：SSH 建连（基础设施）
//   - forwarder.go：隧道长驻运行时（PortForwarder）
//   - service.go：用例编排（校验、启停、测试连接）
//   - dto.go：响应契约与组装
//   - handler.go：参数解析 + 状态码映射 + 路由注册
package portfwd

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"chaos-go/internal/crud"
)

// ErrDBUnavailable 表示数据库单例不可用（极端情况下）。
var ErrDBUnavailable = errors.New("portfwd: database unavailable")

// 领域错误哨兵：repository 与 service 据此表达「不存在」，调用方映射 404。
var (
	ErrRuleNotFound = errors.New("portfwd: 端口转发不存在")
	ErrConnNotFound = errors.New("portfwd: SSH 连接不存在")
)

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
// Status 为「期望运行」标记（供重启恢复 / 自愈使用）；实际运行状态以内存为准（见 dto）。
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

// validateFields 校验端口 / 监听地址 / 目标（纯校验，不查库）。
// 关联 SSH 连接的存在性校验见 service.ValidatePortForward。
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
