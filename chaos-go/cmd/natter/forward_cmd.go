package main

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// version 表示三段式版本号，用于外部工具的最低版本检查。
type version [3]int

func (v version) String() string { return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2]) }

// atLeast 判断当前版本不低于 o。
func (v version) atLeast(o version) bool {
	for i := range v {
		if v[i] != o[i] {
			return v[i] > o[i]
		}
	}
	return true
}

// 各外部工具的版本匹配与最低版本要求。
var (
	iptablesVerRe = regexp.MustCompile(`iptables v([0-9]+)\.([0-9]+)\.([0-9]+)`)
	nftVerRe      = regexp.MustCompile(`nftables v([0-9]+)\.([0-9]+)\.([0-9]+)`)
	gostVerRe     = regexp.MustCompile(`gost v?([0-9]+)\.([0-9]+)`)
	socatVerRe    = regexp.MustCompile(`socat version ([0-9]+)\.([0-9]+)\.([0-9]+)`)

	iptablesMinVer  = version{1, 4, 1}
	iptablesWaitVer = version{1, 4, 20} // 自 1.4.20 起支持 -w 等待锁
	nftMinVer       = version{0, 9, 6}
	gostMinVer      = version{2, 3, 0}
	socatMinVer     = version{1, 7, 2}
)

// parseVersion 从命令输出里提取版本号。
func parseVersion(out string, re *regexp.Regexp) (version, bool) {
	m := re.FindStringSubmatch(out)
	if m == nil {
		return version{}, false
	}
	var v version
	for i := 0; i < 3 && i < len(m)-1; i++ {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return version{}, false
		}
		v[i] = n
	}
	return v, true
}

// concat 拼接命令行，始终返回新切片，避免污染基础命令。
func concat(base, extra []string) []string {
	out := make([]string, 0, len(base)+len(extra))
	out = append(out, base...)
	out = append(out, extra...)
	return out
}

// runCmd 执行外部命令并返回 stdout；stderr 直接透传给终端。
func runCmd(cmd []string) (string, error) {
	if len(cmd) == 0 {
		return "", errors.New("empty command")
	}
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Stderr = os.Stderr
	var buf bytes.Buffer
	c.Stdout = &buf
	err := c.Run()
	return buf.String(), err
}

// checkSysForwardConfig 检查系统是否允许 IP 转发。
func checkSysForwardConfig(tag string) error {
	const fpath = "/proc/sys/net/ipv4/ip_forward"
	b, err := os.ReadFile(fpath)
	if err != nil {
		slog.Warn(tag + ": '" + fpath + "' not found")
		return nil
	}
	if strings.TrimSpace(string(b)) != "1" {
		return errors.New("IP forwarding is not allowed. Please do `sysctl net.ipv4.ip_forward=1`")
	}
	return nil
}

// isRoot 判断当前是否具备 root 权限（非 Unix 恒为 false）。
func isRoot() bool {
	return runtime.GOOS != "windows" && os.Getuid() == 0
}

// extProc 管理转发用的外部子进程。
type extProc struct {
	cmd  *exec.Cmd
	done chan struct{}
}

// startExtProc 启动子进程并等待 1 秒确认它没有立刻退出。
func startExtProc(name string, args ...string) (*extProc, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &extProc{cmd: cmd, done: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		close(p.done)
	}()
	select {
	case <-p.done:
		return nil, fmt.Errorf("%s exited too quickly", name)
	case <-time.After(time.Second):
	}
	return p, nil
}

// stop 终止子进程；已退出则直接返回。
func (p *extProc) stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	select {
	case <-p.done:
		return
	default:
	}
	_ = p.cmd.Process.Kill()
	<-p.done
}

// toolAvailable 检查外部工具是否存在且版本达标。
func toolAvailable(name string, verArgs []string, re *regexp.Regexp, min version) bool {
	out, err := runCmd(append([]string{name}, verArgs...))
	if err != nil {
		return false
	}
	v, ok := parseVersion(out, re)
	if !ok {
		return false
	}
	slog.Debug("fwd: Found " + name + " " + v.String())
	return v.atLeast(min)
}

// ForwardIptables 通过 iptables 的 DNAT（可选 SNAT）实现转发。
type ForwardIptables struct {
	snat    bool
	sudo    bool
	cmd     []string
	rules   [][]string
	currVer version
}

// NewForwardIptables 校验 iptables 可用性并初始化 Natter 链。
func NewForwardIptables(snat, sudo bool) (*ForwardIptables, error) {
	f := &ForwardIptables{snat: snat, sudo: sudo}
	if !f.check() {
		return nil, fmt.Errorf("iptables >= %s not available", iptablesMinVer)
	}
	if f.currVer.atLeast(iptablesWaitVer) {
		f.cmd = append(f.cmd, "-w")
	}
	if err := f.initChain(); err != nil {
		return nil, err
	}
	return f, nil
}

func (f *ForwardIptables) check() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	if f.sudo {
		f.cmd = []string{"sudo", "-n", "iptables"}
	} else {
		f.cmd = []string{"iptables"}
	}
	if !f.sudo && !isRoot() {
		slog.Warn("fwd-iptables: You are not root")
	}
	out, err := runCmd(concat(f.cmd, []string{"--version"}))
	if err != nil {
		return false
	}
	v, ok := parseVersion(out, iptablesVerRe)
	if !ok {
		return false
	}
	f.currVer = v
	slog.Debug("fwd-iptables: Found iptables " + v.String())
	if !v.atLeast(iptablesMinVer) {
		return false
	}
	if _, err := runCmd(concat(f.cmd, []string{"-t", "nat", "--list-rules"})); err != nil {
		return false
	}
	return true
}

func (f *ForwardIptables) initChain() error {
	if _, err := runCmd(concat(f.cmd, []string{"-t", "nat", "--list-rules", "NATTER"})); err == nil {
		return nil
	}
	slog.Debug("fwd-iptables: Creating Natter chain")
	steps := [][]string{
		{"-t", "nat", "-N", "NATTER"},
		{"-t", "nat", "-I", "PREROUTING", "-j", "NATTER"},
		{"-t", "nat", "-I", "OUTPUT", "-j", "NATTER"},
		{"-t", "nat", "-N", "NATTER_SNAT"},
		{"-t", "nat", "-I", "POSTROUTING", "-j", "NATTER_SNAT"},
		{"-t", "nat", "-I", "INPUT", "-j", "NATTER_SNAT"},
	}
	for _, s := range steps {
		if _, err := runCmd(concat(f.cmd, s)); err != nil {
			return err
		}
	}
	return nil
}

// StartForward 添加 DNAT（及可选 SNAT）规则。
func (f *ForwardIptables) StartForward(ip string, port int, toIP string, toPort int, udp bool) error {
	if ip != toIP {
		if err := checkSysForwardConfig("fwd-iptables"); err != nil {
			return err
		}
	}
	if err := checkForwardTarget(ip, port, toIP, toPort); err != nil {
		return err
	}
	proto := protoName(udp)
	slog.Debug("fwd-iptables: Adding rule",
		"from", addr{ip, port}.uri(udp), "to", addr{toIP, toPort}.uri(udp))

	rule := []string{
		"-t", "nat", "-I", "NATTER",
		"-p", proto, "--dst", ip, "--dport", strconv.Itoa(port),
		"-j", "DNAT", "--to-destination", fmt.Sprintf("%s:%d", toIP, toPort),
	}
	if _, err := runCmd(concat(f.cmd, rule)); err != nil {
		f.cleanRules()
		return err
	}
	f.rules = append(f.rules, rule)

	if f.snat {
		rule = []string{
			"-t", "nat", "-I", "NATTER_SNAT",
			"-p", proto, "--dst", toIP, "--dport", strconv.Itoa(toPort),
			"-j", "SNAT", "--to-source", ip,
		}
		if _, err := runCmd(concat(f.cmd, rule)); err != nil {
			f.cleanRules()
			return err
		}
		f.rules = append(f.rules, rule)
	}
	return nil
}

// cleanRules 逆序删除已添加的规则。
func (f *ForwardIptables) cleanRules() {
	for len(f.rules) > 0 {
		last := len(f.rules) - 1
		rule := f.rules[last]
		f.rules = f.rules[:last]
		rm := make([]string, len(rule))
		for i, a := range rule {
			if a == "-I" || a == "-A" {
				rm[i] = "-D"
			} else {
				rm[i] = a
			}
		}
		if _, err := runCmd(concat(f.cmd, rm)); err != nil {
			slog.Error("fwd-iptables: Failed to execute", "cmd", strings.Join(concat(f.cmd, rm), " "), "err", err)
		}
	}
}

// StopForward 清理所有 Natter 规则。
func (f *ForwardIptables) StopForward() {
	slog.Debug("fwd-iptables: Cleaning up Natter rules")
	f.cleanRules()
}

// ForwardNftables 通过 nftables 的 dnat（可选 snat）实现转发。
type ForwardNftables struct {
	snat       bool
	sudo       bool
	cmd        []string
	handle     int
	handleSNAT int
}

// NewForwardNftables 校验 nftables 可用性并初始化 Natter 表。
func NewForwardNftables(snat, sudo bool) (*ForwardNftables, error) {
	f := &ForwardNftables{snat: snat, sudo: sudo, handle: -1, handleSNAT: -1}
	if !f.check() {
		return nil, fmt.Errorf("nftables >= %s not available", nftMinVer)
	}
	if err := f.initTable(); err != nil {
		return nil, err
	}
	return f, nil
}

func (f *ForwardNftables) check() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	if f.sudo {
		f.cmd = []string{"sudo", "-n", "nft"}
	} else {
		f.cmd = []string{"nft"}
	}
	if !f.sudo && !isRoot() {
		slog.Warn("fwd-nftables: You are not root")
	}
	out, err := runCmd(concat(f.cmd, []string{"--version"}))
	if err != nil {
		return false
	}
	v, ok := parseVersion(out, nftVerRe)
	if !ok {
		return false
	}
	slog.Debug("fwd-nftables: Found nftables " + v.String())
	return v.atLeast(nftMinVer)
}

// initialRules Natter 表与链的初始规则集。
const initialRules = `
table ip natter {
    chain natter_dnat { }
    chain natter_snat { }
    chain prerouting {
        type nat hook prerouting priority -105; policy accept;
        jump natter_dnat;
    }
    chain output {
        type nat hook output priority -105; policy accept;
        jump natter_dnat;
    }
    chain postrouting {
        type nat hook postrouting priority 95; policy accept;
        jump natter_snat;
    }
    chain input {
        type nat hook input priority 95; policy accept;
        jump natter_snat;
    }
}
`

func (f *ForwardNftables) initTable() error {
	if _, err := runCmd(concat(f.cmd, []string{"list table ip natter"})); err == nil {
		return nil
	}
	slog.Debug("fwd-nftables: Creating Natter table")
	_, err := runCmd(concat(f.cmd, []string{initialRules}))
	return err
}

// StartForward 插入 dnat（及可选 snat）规则并记住 handle。
func (f *ForwardNftables) StartForward(ip string, port int, toIP string, toPort int, udp bool) error {
	if ip != toIP {
		if err := checkSysForwardConfig("fwd-nftables"); err != nil {
			return err
		}
	}
	if err := checkForwardTarget(ip, port, toIP, toPort); err != nil {
		return err
	}
	proto := protoName(udp)
	slog.Debug("fwd-nftables: Adding rule",
		"from", addr{ip, port}.uri(udp), "to", addr{toIP, toPort}.uri(udp))

	out, err := runCmd(concat(f.cmd, []string{
		"--echo", "--handle",
		fmt.Sprintf("insert rule ip natter natter_dnat ip daddr %s %s dport %d dnat to %s:%d",
			ip, proto, port, toIP, toPort),
	}))
	if err != nil {
		f.clean()
		return err
	}
	handle, err := parseHandle(out)
	if err != nil {
		f.clean()
		return err
	}
	f.handle = handle

	if f.snat {
		out, err := runCmd(concat(f.cmd, []string{
			"--echo", "--handle",
			fmt.Sprintf("insert rule ip natter natter_snat ip daddr %s %s dport %d snat to %s",
				toIP, proto, toPort, ip),
		}))
		if err != nil {
			f.clean()
			return err
		}
		handle, err := parseHandle(out)
		if err != nil {
			f.clean()
			return err
		}
		f.handleSNAT = handle
	}
	return nil
}

// handleRe 从 `nft --echo --handle` 的输出里提取规则 handle。
var handleRe = regexp.MustCompile(`# handle ([0-9]+)`)

func parseHandle(out string) (int, error) {
	m := handleRe.FindStringSubmatch(out)
	if m == nil {
		return 0, errors.New("unknown nftables handle")
	}
	return strconv.Atoi(m[1])
}

func (f *ForwardNftables) clean() {
	slog.Debug("fwd-nftables: Cleaning up Natter rules")
	if f.handle > 0 {
		_, _ = runCmd(concat(f.cmd, []string{
			fmt.Sprintf("delete rule ip natter natter_dnat handle %d", f.handle)}))
		f.handle = -1
	}
	if f.handleSNAT > 0 {
		_, _ = runCmd(concat(f.cmd, []string{
			fmt.Sprintf("delete rule ip natter natter_snat handle %d", f.handleSNAT)}))
		f.handleSNAT = -1
	}
}

// StopForward 删除已插入的规则。
func (f *ForwardNftables) StopForward() { f.clean() }

// ForwardGost 通过外部 gost 进程转发。
type ForwardGost struct {
	proc *extProc
}

// NewForwardGost 校验 gost 版本。
func NewForwardGost() (*ForwardGost, error) {
	if !toolAvailable("gost", []string{"-V"}, gostVerRe, gostMinVer) {
		return nil, fmt.Errorf("gost >= %d.%d not available", gostMinVer[0], gostMinVer[1])
	}
	return &ForwardGost{}, nil
}

// StartForward 启动 gost 进程。
func (f *ForwardGost) StartForward(ip string, port int, toIP string, toPort int, udp bool) error {
	if err := checkForwardTarget(ip, port, toIP, toPort); err != nil {
		return err
	}
	slog.Debug("fwd-gost: Starting gost",
		"from", addr{ip, port}.uri(udp), "to", addr{toIP, toPort}.uri(udp))
	arg := fmt.Sprintf("-L=%s://:%d/%s:%d", protoName(udp), port, toIP, toPort)
	if udp {
		arg += fmt.Sprintf("?ttl=%ds", int(fwdUDPTimeout/time.Second))
	}
	proc, err := startExtProc("gost", arg)
	if err != nil {
		return err
	}
	f.proc = proc
	return nil
}

// StopForward 停止 gost 进程。
func (f *ForwardGost) StopForward() {
	slog.Debug("fwd-gost: Stopping gost")
	f.proc.stop()
	f.proc = nil
}

// ForwardSocat 通过外部 socat 进程转发。
type ForwardSocat struct {
	proc *extProc
}

// NewForwardSocat 校验 socat 版本。
func NewForwardSocat() (*ForwardSocat, error) {
	if !toolAvailable("socat", []string{"-V"}, socatVerRe, socatMinVer) {
		return nil, fmt.Errorf("socat >= %s not available", socatMinVer)
	}
	return &ForwardSocat{}, nil
}

// StartForward 启动 socat 进程。
func (f *ForwardSocat) StartForward(ip string, port int, toIP string, toPort int, udp bool) error {
	if err := checkForwardTarget(ip, port, toIP, toPort); err != nil {
		return err
	}
	slog.Debug("fwd-socat: Starting socat",
		"from", addr{ip, port}.uri(udp), "to", addr{toIP, toPort}.uri(udp))
	proto := strings.ToUpper(protoName(udp))
	args := []string{}
	if udp {
		args = append(args, fmt.Sprintf("-T%d", int(fwdUDPTimeout/time.Second)))
	}
	args = append(args,
		fmt.Sprintf("%s4-LISTEN:%d,reuseaddr,fork,max-children=%d", proto, port, fwdMaxConcurrent),
		fmt.Sprintf("%s4:%s:%d", proto, toIP, toPort),
	)
	proc, err := startExtProc("socat", args...)
	if err != nil {
		return err
	}
	f.proc = proc
	return nil
}

// StopForward 停止 socat 进程。
func (f *ForwardSocat) StopForward() {
	slog.Debug("fwd-socat: Stopping socat")
	f.proc.stop()
	f.proc = nil
}

// protoName 返回协议名（小写，用于 iptables / nftables / gost）。
func protoName(udp bool) string {
	if udp {
		return "udp"
	}
	return "tcp"
}

// newForwarder 按方法名创建转发实现。
func newForwarder(method string) (Forwarder, error) {
	switch method {
	case "none":
		return &ForwardNone{}, nil
	case "test":
		return &ForwardTestServer{}, nil
	case "socket":
		return NewForwardSocket(), nil
	case "iptables":
		return NewForwardIptables(false, false)
	case "sudo-iptables":
		return NewForwardIptables(false, true)
	case "iptables-snat":
		return NewForwardIptables(true, false)
	case "sudo-iptables-snat":
		return NewForwardIptables(true, true)
	case "nftables":
		return NewForwardNftables(false, false)
	case "sudo-nftables":
		return NewForwardNftables(false, true)
	case "nftables-snat":
		return NewForwardNftables(true, false)
	case "sudo-nftables-snat":
		return NewForwardNftables(true, true)
	case "gost":
		return NewForwardGost()
	case "socat":
		return NewForwardSocat()
	}
	return nil, fmt.Errorf("unknown method name: %s", method)
}
