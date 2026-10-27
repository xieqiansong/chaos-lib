package portfwd

import (
	"net/http"

	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/httpx"
	renv "chaos-go/internal/framework/resp"

	"chaos-go/internal/framework/web"
)

var ruleErrRules = []httpx.ErrRule{
	{Err: ErrRuleNotFound, Status: http.StatusNotFound, Msg: "端口转发不存在"},
	{Err: ErrAlreadyRunning, Status: http.StatusBadRequest},
	{Err: ErrNotRunning, Status: http.StatusBadRequest},
	{Err: ErrConnRequired, Status: http.StatusBadRequest},
	{Err: ErrInvalidRule, Status: http.StatusBadRequest},
}

var connErrRules = []httpx.ErrRule{
	{Err: ErrConnNotFound, Status: http.StatusNotFound, Msg: "SSH 连接不存在"},
}

var sshOpts = crud.Opts[SshConnection]{
	Searchable:   []string{"name", "host", "username", "remark"},
	Sortable:     []string{"id", "name", "host", "username"},
	ToResponse:   sshConnsToResponse,
	BeforeCreate: ValidateSshConnForCreate,
	AfterUpdate:  ValidateSshConnForUpdate,
	AfterDelete:  GuardSshConnDeletion,
}
var pfOpts = crud.Opts[PortForwarding]{
	Searchable:   []string{"name", "direction", "target_host", "remark"},
	Sortable:     []string{"id", "name", "direction", "port", "target_port"},
	Protected:    []string{"Status"},
	ToResponse:   PortForwardingsToResponse,
	BeforeCreate: PreparePortForwardForCreate,
	AfterUpdate:  GuardPortForwardUpdate,
	AfterDelete:  StopPortForwardOnDelete,
}
var pfToggle = &crud.ToggleOpts{
	Setter:   func(id int, status bool) (any, error) { return SetForwardStatus(id, status) },
	ErrRules: ruleErrRules,
}

func RegisterV1(rg *web.RouterGroup) {
	ssh := crud.RegisterActions[SshConnection](rg, "ssh-conns", sshOpts, nil)
	ssh.POST("/test", testSshV1)
	crud.RegisterActions[PortForwarding](rg, "port-forwards", pfOpts, pfToggle)
}

func testSshConnection(c *web.Context) {
	id, ok := httpx.ParseID(c)
	if !ok {
		return
	}
	doTestSsh(c, id)
}

func testSshV1(c *web.Context) {
	var req struct {
		ID int `json:"id"`
	}
	if err := envelope.Bind(c, &req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	doTestSsh(c, req.ID)
}

func doTestSsh(c *web.Context, id int) {
	result, err := TestSshConn(id)
	if err != nil {
		httpx.MapError(c, err, connErrRules, http.StatusBadRequest)
		return
	}
	renv.Success(c, result)
}
