package portfwd

// PortForwardingResponse 对外 DTO；Status 反映内存中实际运行状态（非库里的期望值）。
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

// SshTestResult SSH 连通性测试结果。
type SshTestResult struct {
	SshAddr       string `json:"sshAddr"`
	ServerVersion string `json:"serverVersion"`
	RemoteAddr    string `json:"remoteAddr"`
}

// toPortForwardingResponse 组装单条转发规则的响应；运行状态由调用方注入（model 不感知运行时）。
func toPortForwardingResponse(pf *PortForwarding, running bool, lastErr string) PortForwardingResponse {
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

// toSshConnectionResponse 组装单条 SSH 连接的响应（凭据仅回显是否配置）。
func toSshConnectionResponse(conn *SshConnection) SshConnectionResponse {
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
		out = append(out, toSshConnectionResponse(c))
	}
	return out
}
