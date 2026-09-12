package portfwd

import (
	"chaos-go/config"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/ssh"
)

// ── 模型 ──────────────────────────────────────────────────────────

const (
	AuthTypePassword = "password"
	AuthTypeKey      = "key"
)

// SshConnection SSH 连接信息。
// 凭据（Password / PrivateKey / Passphrase）只用于后端建连，一律不经 API 返回、不写入日志。
type SshConnection struct {
	Id         int `gorm:"primaryKey"`
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

// SshConnectionResponse 对外 DTO：只暴露「是否已配置凭据」，绝不返回凭据明文。
type SshConnectionResponse struct {
	Id            int
	Name          string
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
		Id:            conn.Id,
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

// ── 连接构建 ──────────────────────────────────────────────────────

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

// ── 校验 ──────────────────────────────────────────────────────────

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

// ── Handlers ──────────────────────────────────────────────────────

func GetSshConnections(c *gin.Context) {
	var conns []SshConnection
	config.GetDB().Order("id ASC").Find(&conns)
	responses := make([]SshConnectionResponse, 0, len(conns))
	for i := range conns {
		responses = append(responses, conns[i].toResponse())
	}
	c.JSON(200, responses)
}

func CreateSshConnection(c *gin.Context) {
	var req struct {
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
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	conn := SshConnection{
		Name: req.Name, Host: req.Host, Port: req.Port, Username: req.Username,
		AuthType: req.AuthType, Password: req.Password, PrivateKey: req.PrivateKey,
		Passphrase: req.Passphrase, Remark: req.Remark,
	}
	if err := conn.validateForSave(true); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if result := config.GetDB().Create(&conn); result.Error != nil {
		c.JSON(500, gin.H{"error": "创建失败: " + result.Error.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "创建成功", "data": conn.toResponse()})
}

func UpdateSshConnection(c *gin.Context) {
	id := c.Param("id")
	var conn SshConnection
	if result := config.GetDB().First(&conn, "id = ?", id); result.Error != nil {
		c.JSON(404, gin.H{"error": "SSH 连接不存在"})
		return
	}
	// 凭据字段为空表示保持原值，避免前端回显
	var req struct {
		Name       *string
		Host       *string
		Port       *int
		Username   *string
		AuthType   *string
		Password   *string
		PrivateKey *string
		Passphrase *string
		Remark     *string
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Name != nil {
		conn.Name = *req.Name
	}
	if req.Host != nil {
		conn.Host = *req.Host
	}
	if req.Port != nil {
		conn.Port = *req.Port
	}
	if req.Username != nil {
		conn.Username = *req.Username
	}
	if req.AuthType != nil {
		conn.AuthType = *req.AuthType
	}
	if req.Password != nil && *req.Password != "" {
		conn.Password = *req.Password
	}
	if req.PrivateKey != nil && *req.PrivateKey != "" {
		conn.PrivateKey = *req.PrivateKey
	}
	if req.Passphrase != nil && *req.Passphrase != "" {
		conn.Passphrase = *req.Passphrase
	}
	if req.Remark != nil {
		conn.Remark = *req.Remark
	}
	if err := conn.validateForSave(true); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if result := config.GetDB().Save(&conn); result.Error != nil {
		c.JSON(500, gin.H{"error": "更新失败: " + result.Error.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "更新成功", "data": conn.toResponse()})
}

func DeleteSshConnection(c *gin.Context) {
	id := c.Param("id")
	var conn SshConnection
	if result := config.GetDB().First(&conn, "id = ?", id); result.Error != nil {
		c.JSON(404, gin.H{"error": "SSH 连接不存在"})
		return
	}
	var count int64
	config.GetDB().Model(&PortForwarding{}).Where("ssh_connection_id = ?", conn.Id).Count(&count)
	if count > 0 {
		c.JSON(400, gin.H{"error": fmt.Sprintf("该 SSH 连接被 %d 条转发规则引用，请先删除相关规则", count)})
		return
	}
	if result := config.GetDB().Delete(&conn); result.Error != nil {
		c.JSON(500, gin.H{"error": "删除失败: " + result.Error.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "删除成功"})
}

func TestSshConnection(c *gin.Context) {
	id := c.Param("id")
	var conn SshConnection
	if result := config.GetDB().First(&conn, "id = ?", id); result.Error != nil {
		c.JSON(404, gin.H{"error": "SSH 连接不存在"})
		return
	}
	client, err := conn.dial()
	if err != nil {
		slog.Warn("SSH 连接测试失败", "sshAddr", conn.sshAddr(), "username", conn.Username, "err", err)
		c.JSON(400, gin.H{"error": "连接失败: " + err.Error()})
		return
	}
	defer client.Close()
	c.JSON(200, gin.H{
		"message": "连接成功",
		"data": gin.H{
			"sshAddr":       conn.sshAddr(),
			"serverVersion": string(client.ServerVersion()),
			"remoteAddr":    client.RemoteAddr().String(),
		},
	})
}
