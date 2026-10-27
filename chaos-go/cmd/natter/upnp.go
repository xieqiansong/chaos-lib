package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strings"
	"time"
)

const (
	upnpSockTimeout = 3 * time.Second
	upnpSSDPTimeout = 1 * time.Second
)

var upnpForwardTypes = map[string]bool{
	"urn:schemas-upnp-org:service:WANIPConnection:1":  true,
	"urn:schemas-upnp-org:service:WANIPConnection:2":  true,
	"urn:schemas-upnp-org:service:WANPPPConnection:1": true,
}

var (
	serviceRe     = regexp.MustCompile(`(?s)<service\s*>(.*?)</service\s*>`)
	serviceTypeRe = regexp.MustCompile(`<serviceType\s*>([^<]*?)</serviceType\s*>`)
	serviceIDRe   = regexp.MustCompile(`<serviceId\s*>([^<]*?)</serviceId\s*>`)
	scpdURLRe     = regexp.MustCompile(`<SCPDURL\s*>([^<]*?)</SCPDURL\s*>`)
	controlURLRe  = regexp.MustCompile(`<controlURL\s*>([^<]*?)</controlURL\s*>`)
	eventSubURLRe = regexp.MustCompile(`<eventSubURL\s*>([^<]*?)</eventSubURL\s*>`)

	locationRe = regexp.MustCompile(`LOCATION: *(http://[^\[]\S+)\s+`)

	errCodeRe = regexp.MustCompile(`<errorCode\s*>([^<]*?)</errorCode\s*>`)
	errDescRe = regexp.MustCompile(`<errorDescription\s*>([^<]*?)</errorDescription\s*>`)
)

type UPnPService struct {
	device      *UPnPDevice
	ServiceType string
	ServiceID   string
	SCPDURL     string
	ControlURL  string
	EventSubURL string

	bindIP    string
	bindIface string
	timeout   time.Duration
}

func (s *UPnPService) isValid() bool {
	return s.ServiceType != "" && s.ServiceID != "" && s.ControlURL != ""
}

func (s *UPnPService) isForward() bool {
	return upnpForwardTypes[s.ServiceType] && s.ServiceID != "" && s.ControlURL != ""
}

func (s *UPnPService) ForwardPort(ctx context.Context, host string, port int, destHost string, destPort int, udp bool, duration int) (bool, error) {
	if !s.isForward() {
		return false, fmt.Errorf("unsupported service type: %s", s.ServiceType)
	}
	proto := "TCP"
	if udp {
		proto = "UDP"
	}
	ctlHost, ctlPort, ctlPath, err := splitURL(s.ControlURL)
	if err != nil {
		return false, err
	}
	content := fmt.Sprintf(
		`<?xml version="1.0" encoding="utf-8"?>`+"\r\n"+
			`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"`+"\r\n"+
			`  s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">`+"\r\n"+
			`  <s:Body>`+"\r\n"+
			`    <m:AddPortMapping xmlns:m="%s">`+"\r\n"+
			`      <NewRemoteHost>%s</NewRemoteHost>`+"\r\n"+
			`      <NewExternalPort>%d</NewExternalPort>`+"\r\n"+
			`      <NewProtocol>%s</NewProtocol>`+"\r\n"+
			`      <NewInternalPort>%d</NewInternalPort>`+"\r\n"+
			`      <NewInternalClient>%s</NewInternalClient>`+"\r\n"+
			`      <NewEnabled>1</NewEnabled>`+"\r\n"+
			`      <NewPortMappingDescription>%s</NewPortMappingDescription>`+"\r\n"+
			`      <NewLeaseDuration>%d</NewLeaseDuration>`+"\r\n"+
			`    </m:AddPortMapping>`+"\r\n"+
			`  </s:Body>`+"\r\n"+
			`</s:Envelope>`+"\r\n",
		s.ServiceType, host, port, proto, destPort, destHost, "Natter", duration)

	data := fmt.Sprintf(
		"POST %s HTTP/1.1\r\n"+
			"Host: %s:%d\r\n"+
			"User-Agent: curl/8.0.0 (Natter)\r\n"+
			"Accept: */*\r\n"+
			"SOAPAction: \"%s#AddPortMapping\"\r\n"+
			"Content-Type: text/xml\r\n"+
			"Content-Length: %d\r\n"+
			"Connection: close\r\n"+
			"\r\n"+
			"%s", ctlPath, ctlHost, ctlPort, s.ServiceType, len(content), content)

	o := sockOpts{bindIP: s.bindIP, iface: s.bindIface, timeout: s.timeout}
	resp, err := rawHTTP(ctx, o, addr{ctlHost, ctlPort}, []byte(data))
	if err != nil {
		return false, err
	}
	r := string(resp)
	errCode, errMsg := "", ""
	if m := errCodeRe.FindStringSubmatch(r); m != nil {
		errCode = strings.TrimSpace(m[1])
	}
	if m := errDescRe.FindStringSubmatch(r); m != nil {
		errMsg = strings.TrimSpace(m[1])
	}
	if errCode != "" || errMsg != "" {
		slog.Error("upnp: Error from service",
			"service", s.ServiceType, "device", s.device.ipaddr, "code", errCode, "msg", errMsg)
		return false, nil
	}
	return true, nil
}

type UPnPDevice struct {
	ipaddr     string
	xmlURLs    []string
	services   []*UPnPService
	forwardSrv *UPnPService

	bindIP    string
	bindIface string
	timeout   time.Duration
}

func (d *UPnPDevice) loadServices(ctx context.Context) {
	if len(d.services) > 0 {
		return
	}
	byID := map[string]*UPnPService{}
	for _, u := range d.xmlURLs {
		for id, srv := range d.getSrvDict(ctx, u) {
			byID[id] = srv
		}
	}
	for _, srv := range byID {
		d.services = append(d.services, srv)
		if srv.isForward() {
			d.forwardSrv = srv
			break
		}
	}
}

func (d *UPnPDevice) httpGet(ctx context.Context, u string) ([]byte, error) {
	host, port, path, err := splitURL(u)
	if err != nil {
		return nil, err
	}
	req := fmt.Sprintf(
		"GET %s HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"User-Agent: curl/8.0.0 (Natter)\r\n"+
			"Accept: */*\r\n"+
			"Connection: close\r\n"+
			"\r\n", path, host)
	o := sockOpts{bindIP: d.bindIP, iface: d.bindIface, timeout: d.timeout}
	resp, err := rawHTTP(ctx, o, addr{host, port}, []byte(req))
	if err != nil {
		return nil, err
	}
	if len(resp) < 5 || string(resp[:5]) != "HTTP/" {
		return nil, fmt.Errorf("invalid response from HTTP server: %s", u)
	}
	body, ok := httpBody(resp)
	if !ok {
		return nil, fmt.Errorf("invalid response from HTTP server: %s", u)
	}
	return body, nil
}

func (d *UPnPDevice) getSrvDict(ctx context.Context, u string) map[string]*UPnPService {
	out := map[string]*UPnPService{}
	raw, err := d.httpGet(ctx, u)
	if err != nil {
		slog.Warn("upnp: failed to load service", "url", u, "err", err)
		return out
	}
	xmlContent := string(raw)
	for _, srvStr := range serviceRe.FindAllStringSubmatch(xmlContent, -1) {
		block := srvStr[1]
		srv := &UPnPService{
			device:    d,
			bindIP:    d.bindIP,
			bindIface: d.bindIface,
			timeout:   d.timeout,
		}
		if m := serviceTypeRe.FindStringSubmatch(block); m != nil {
			srv.ServiceType = strings.TrimSpace(m[1])
		}
		if m := serviceIDRe.FindStringSubmatch(block); m != nil {
			srv.ServiceID = strings.TrimSpace(m[1])
		}
		if m := scpdURLRe.FindStringSubmatch(block); m != nil {
			srv.SCPDURL = fullURL(strings.TrimSpace(m[1]), u)
		}
		if m := controlURLRe.FindStringSubmatch(block); m != nil {
			srv.ControlURL = fullURL(strings.TrimSpace(m[1]), u)
		}
		if m := eventSubURLRe.FindStringSubmatch(block); m != nil {
			srv.EventSubURL = fullURL(strings.TrimSpace(m[1]), u)
		}
		if srv.isValid() {
			out[srv.ServiceID] = srv
		}
	}
	return out
}

type UPnPClient struct {
	ssdpAddr    addr
	router      *UPnPDevice
	bindIP      string
	bindIface   string
	sockTimeout time.Duration

	fwd struct {
		host     string
		port     int
		destHost string
		destPort int
		udp      bool
		duration int
		started  bool
	}
}

func NewUPnPClient(bindIP, bindIface string) *UPnPClient {
	return &UPnPClient{
		ssdpAddr:    addr{"239.255.255.250", 1900},
		bindIP:      bindIP,
		bindIface:   bindIface,
		sockTimeout: upnpSockTimeout,
	}
}

func (c *UPnPClient) DiscoverRouter(ctx context.Context) (*UPnPDevice, error) {
	devs, err := c.discover(ctx)
	if err != nil {
		return nil, err
	}
	var routers []*UPnPDevice
	for _, d := range devs {
		if d.forwardSrv != nil {
			routers = append(routers, d)
		}
	}
	switch {
	case len(routers) == 0:
		c.router = nil
	case len(routers) > 1:
		slog.Warn("upnp: multiple routers found", "count", len(routers))
		c.router = routers[0]
	default:
		c.router = routers[0]
	}
	return c.router, nil
}

func (c *UPnPClient) discover(ctx context.Context) ([]*UPnPDevice, error) {
	bind := listenAddr(c.bindIP, 0)
	lc := newListenConfig(sockOpts{reuse: true, bindIP: c.bindIP, iface: c.bindIface, udp: true})
	pc, err := lc.ListenPacket(ctx, "udp4", bind)
	if err != nil {
		return nil, err
	}
	defer pc.Close()

	target := &net.UDPAddr{IP: net.ParseIP(c.ssdpAddr.Host), Port: c.ssdpAddr.Port}
	host := fmt.Sprintf("%s:%d", c.ssdpAddr.Host, c.ssdpAddr.Port)
	dat01 := fmt.Sprintf(
		"M-SEARCH * HTTP/1.1\r\nST: ssdp:all\r\nMX: 2\r\nMAN: \"ssdp:discover\"\r\nHOST: %s\r\n\r\n", host)
	dat02 := fmt.Sprintf(
		"M-SEARCH * HTTP/1.1\r\nST: upnp:rootdevice\r\nMX: 2\r\nMAN: \"ssdp:discover\"\r\nHOST: %s\r\n\r\n", host)
	_, _ = pc.WriteTo([]byte(dat01), target)
	_, _ = pc.WriteTo([]byte(dat02), target)

	urlsByIP := map[string]map[string]struct{}{}
	deadline := time.Now().Add(upnpSSDPTimeout)
	buf := make([]byte, 4096)
	for {
		if err := pc.SetReadDeadline(deadline); err != nil {
			return nil, err
		}
		n, from, err := pc.ReadFrom(buf)
		if err != nil {
			if isTimeoutErr(err) {
				break
			}
			return nil, err
		}
		m := locationRe.FindStringSubmatch(string(buf[:n]))
		if m == nil {
			continue
		}
		udpAddr, ok := from.(*net.UDPAddr)
		if !ok {
			continue
		}
		ipaddr := udpAddr.IP.String()
		location := m[1]
		slog.Debug("upnp: Got URL " + location)
		if urlsByIP[ipaddr] == nil {
			urlsByIP[ipaddr] = map[string]struct{}{}
		}
		urlsByIP[ipaddr][location] = struct{}{}
	}

	var devs []*UPnPDevice
	for ipaddr, urlSet := range urlsByIP {
		urls := make([]string, 0, len(urlSet))
		for u := range urlSet {
			urls = append(urls, u)
		}
		d := &UPnPDevice{
			ipaddr:    ipaddr,
			xmlURLs:   urls,
			bindIP:    c.bindIP,
			bindIface: c.bindIface,
			timeout:   c.sockTimeout,
		}
		d.loadServices(ctx)
		devs = append(devs, d)
	}
	return devs, nil
}

func (c *UPnPClient) Forward(ctx context.Context, host string, port int, destHost string, destPort int, udp bool, duration int) (bool, error) {
	if c.router == nil || c.router.forwardSrv == nil {
		return false, fmt.Errorf("no router is available")
	}
	ok, err := c.router.forwardSrv.ForwardPort(ctx, host, port, destHost, destPort, udp, duration)
	if err != nil {
		return false, err
	}
	c.fwd.host, c.fwd.port = host, port
	c.fwd.destHost, c.fwd.destPort = destHost, destPort
	c.fwd.udp, c.fwd.duration = udp, duration
	c.fwd.started = true
	return ok, nil
}

func (c *UPnPClient) Renew(ctx context.Context) (bool, error) {
	if !c.fwd.started {
		return false, fmt.Errorf("UPnP forward not started")
	}
	ok, err := c.router.forwardSrv.ForwardPort(ctx,
		c.fwd.host, c.fwd.port, c.fwd.destHost, c.fwd.destPort, c.fwd.udp, c.fwd.duration)
	if err != nil {
		return false, err
	}
	slog.Debug("upnp: OK")
	return ok, nil
}
