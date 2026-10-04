package portfwd

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// 启停用法的失败原因（调用方据此区分 400 与 500）。
var (
	ErrAlreadyRunning = errors.New("portfwd: 该端口转发已启动")
	ErrNotRunning     = errors.New("portfwd: 该端口转发未启动")
	ErrConnRequired   = errors.New("portfwd: SSH 连接不存在，无法启动")
	ErrInvalidRule    = errors.New("portfwd: 转发规则不合法")
)

// ── SSH 连接 ────────────────────────────────────────────────────

// ValidateSshConnForCreate 新建 SSH 连接前校验（要求凭据已配置）。
func ValidateSshConnForCreate(conn *SshConnection) error {
	return conn.validateForSave(true)
}

// ValidateSshConnForUpdate 更新 SSH 连接前校验（凭据可留空表示不改）。
func ValidateSshConnForUpdate(conn *SshConnection) error {
	return conn.validateForSave(false)
}

// GuardSshConnDeletion 删除 SSH 连接前拦截：仍被转发规则引用则拒绝（回滚软删）。
func GuardSshConnDeletion(conn *SshConnection) error {
	count, err := CountPortForwardingsByConn(conn.ID)
	if err != nil {
		return fmt.Errorf("校验关联转发失败: %v", err)
	}
	if count > 0 {
		return fmt.Errorf("该 SSH 连接被 %d 条转发规则引用，请先删除相关规则", count)
	}
	return nil
}

// TestSshConn 测试 SSH 连接可达性与凭据有效性（不启动端口转发）。
func TestSshConn(id int) (*SshTestResult, error) {
	conn, err := FindSshConnByID(id)
	if err != nil {
		return nil, err
	}
	client, err := conn.dial()
	if err != nil {
		slog.Warn("SSH 连接测试失败", "sshAddr", conn.sshAddr(), "username", conn.Username, "err", err)
		return nil, fmt.Errorf("连接失败: %w", err)
	}
	defer client.Close()
	return &SshTestResult{
		SshAddr:       conn.sshAddr(),
		ServerVersion: string(client.ServerVersion()),
		RemoteAddr:    client.RemoteAddr().String(),
	}, nil
}

// ── 端口转发 ────────────────────────────────────────────────────

// ValidatePortForward 校验转发规则：规范化 + 字段校验 + 关联 SSH 连接存在性（直连除外），
// 名称留空时按方向自动生成。
func ValidatePortForward(pf *PortForwarding) error {
	if err := pf.normalize(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRule, err)
	}
	if err := pf.validateFields(); err != nil {
		return err
	}
	// 直接转发不经 SSH 隧道，无需关联 SSH 连接；其余方向必须关联一条已存在的连接。
	if pf.Direction != DirectionDirect {
		if pf.SshConnectionId <= 0 {
			return fmt.Errorf("请选择 SSH 连接")
		}
		exists, err := SshConnExists(pf.SshConnectionId)
		if err != nil {
			return fmt.Errorf("校验 SSH 连接失败: %v", err)
		}
		if !exists {
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

// PreparePortForwardForCreate 新建转发规则前规范化并校验（新建一律未启动）。
func PreparePortForwardForCreate(pf *PortForwarding) error {
	pf.Status = false
	return ValidatePortForward(pf)
}

// GuardPortForwardUpdate 更新前校验：运行中的规则禁止修改，需先停止。
func GuardPortForwardUpdate(pf *PortForwarding) error {
	if running, _ := GlobalPortForwarder.Status(pf.ID); running {
		return fmt.Errorf("该转发正在运行，请先停止后再修改")
	}
	return ValidatePortForward(pf)
}

// StopPortForwardOnDelete 删除（软删）前若正在运行则先停掉隧道。
func StopPortForwardOnDelete(pf *PortForwarding) error {
	if running, _ := GlobalPortForwarder.Status(pf.ID); running {
		if err := GlobalPortForwarder.RemoveForward(pf.ID); err != nil {
			return fmt.Errorf("停止端口转发失败: %v", err)
		}
	}
	return nil
}

// SetForwardStatus 启停单条转发规则：加载规则与关联连接 → 建 / 撤隧道 → 落库期望状态，
// 返回以「内存实际运行状态」为准的响应视图。
func SetForwardStatus(id int, enable bool) (*PortForwardingResponse, error) {
	rule, err := FindPortForwardingByID(id)
	if err != nil {
		return nil, err
	}
	if err := rule.normalize(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRule, err)
	}
	running, _ := GlobalPortForwarder.Status(rule.ID)
	if enable {
		if running {
			return nil, ErrAlreadyRunning
		}
		// 直接转发不经 SSH 隧道，无需加载 SSH 连接；其余方向必须存在对应连接。
		var conn *SshConnection
		if rule.Direction != DirectionDirect {
			loaded, err := FindSshConnByID(rule.SshConnectionId)
			if err != nil {
				return nil, ErrConnRequired
			}
			conn = loaded
		}
		if err := GlobalPortForwarder.AddForward(rule, conn); err != nil {
			return nil, fmt.Errorf("启动端口转发失败: %w", err)
		}
		rule.Status = true
	} else {
		if !running {
			return nil, ErrNotRunning
		}
		if err := GlobalPortForwarder.RemoveForward(rule.ID); err != nil {
			return nil, fmt.Errorf("停止端口转发失败: %w", err)
		}
		rule.Status = false
	}
	if err := SavePortForwarding(rule); err != nil {
		return nil, fmt.Errorf("更新状态失败: %w", err)
	}
	running, lastErr := GlobalPortForwarder.Status(rule.ID)
	resp := toPortForwardingResponse(rule, running, lastErr)
	return &resp, nil
}

// PortForwardingsToResponse 整批把 []*PortForwarding 转成响应 DTO（供 crud.ToResponse 调用）。
// 运行状态取自内存转发器，故放在用例层而非 dto。
func PortForwardingsToResponse(rows []*PortForwarding) any {
	out := make([]PortForwardingResponse, 0, len(rows))
	for _, r := range rows {
		// 存量行可能没有 direction，读路径同样按 local 兜底
		_ = r.normalize()
		running, lastErr := GlobalPortForwarder.Status(r.ID)
		out = append(out, toPortForwardingResponse(r, running, lastErr))
	}
	return out
}
