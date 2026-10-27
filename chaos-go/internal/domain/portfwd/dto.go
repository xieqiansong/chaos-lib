package portfwd

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

type SshTestResult struct {
	SshAddr       string `json:"sshAddr"`
	ServerVersion string `json:"serverVersion"`
	RemoteAddr    string `json:"remoteAddr"`
}

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

func sshConnsToResponse(rows []*SshConnection) any {
	out := make([]SshConnectionResponse, 0, len(rows))
	for _, c := range rows {
		out = append(out, toSshConnectionResponse(c))
	}
	return out
}
