package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type config struct {
	ControlURL       string
	Token            string
	AdvertiseAddress string
	Region           string
	Version          string
	ConfigInterval   time.Duration
	HeartbeatInterval time.Duration
	ReportInterval   time.Duration
}

type apiClient struct {
	baseURL string
	token   string
	client  *http.Client
}

type apiResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
}

type device struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	GroupID         int    `json:"groupId"`
	GroupName       string `json:"groupName"`
	Type            string `json:"type"`
	Status          string `json:"status"`
	Address         string `json:"address"`
	Region          string `json:"region"`
	Version         string `json:"version"`
	Load            string `json:"load"`
	LatencyMs       int    `json:"latencyMs"`
	ConnectionCount int    `json:"connectionCount"`
	InboundTraffic  string `json:"inboundTraffic"`
	OutboundTraffic string `json:"outboundTraffic"`
	LastHeartbeat   string `json:"lastHeartbeat"`
	LastSeen        string `json:"lastSeen"`
	Enabled         bool   `json:"enabled"`
	ConfigVersion   int    `json:"configVersion"`
}

type controlEnvelope struct {
	Device        device       `json:"device"`
	ConfigVersion int          `json:"configVersion"`
	Rules         []agentRule  `json:"rules"`
	GeneratedAt   string      `json:"generatedAt"`
	ServerTime    string      `json:"serverTime"`
}

type agentRule struct {
	Role        string        `json:"role"`
	Rule        forwardRule   `json:"rule"`
	ExitDevices []device      `json:"exitDevices"`
}

type forwardRule struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	EntryGroupID      int    `json:"entryGroupId"`
	EntryGroupName    string `json:"entryGroupName"`
	ListenHost        string `json:"listenHost"`
	ListenPort        string `json:"listenPort"`
	ExitGroupID       int    `json:"exitGroupId"`
	ExitGroupName     string `json:"exitGroupName"`
	TargetHost        string `json:"targetHost"`
	TargetPort        string `json:"targetPort"`
	Strategy          string `json:"strategy"`
	Protocol          string `json:"protocol"`
	ProxyProtocol     string `json:"proxyProtocol"`
	ProxyProtocolMode string `json:"proxyProtocolMode"`
	Enabled           bool   `json:"enabled"`
}

type heartbeatPayload struct {
	Status          string `json:"status"`
	Address         string `json:"address"`
	Region          string `json:"region"`
	Version         string `json:"version"`
	Load            string `json:"load"`
	LatencyMs       int    `json:"latencyMs"`
	ConnectionCount int    `json:"connectionCount"`
	InboundTraffic  string `json:"inboundTraffic"`
	OutboundTraffic string `json:"outboundTraffic"`
}

type connectionReport struct {
	SourceIP        string `json:"sourceIp"`
	SourcePort      int    `json:"sourcePort"`
	RuleID          int    `json:"ruleId"`
	RuleName        string `json:"ruleName"`
	Protocol        string `json:"protocol"`
	RealIPSource    string `json:"realIpSource"`
	ConnectionCount int    `json:"connectionCount"`
	Country         string `json:"country"`
	Remark          string `json:"remark"`
}

type connectionReporter struct {
	mu      sync.Mutex
	active  map[string]connectionReport
	total   int64
}

type relayManager struct {
	cfg      config
	reporter *connectionReporter
	tunnel   *tunnelClient
	mu       sync.Mutex
	relays   map[int]*runningRelay
}

type runningRelay struct {
	key    string
	rule   forwardRule
	cancel context.CancelFunc
}

type tunnelFrame struct {
	Type           byte
	StreamID       uint64
	SourceDeviceID uint32
	TargetDeviceID uint32
	Payload        []byte
}

type tunnelClient struct {
	cfg      config
	reporter *connectionReporter
	mu       sync.RWMutex
	conn     net.Conn
	writerMu sync.Mutex
	streams  map[uint64]*tunnelStream
	nextID   uint64
}

type tunnelStream struct {
	id       uint64
	peerID   int
	client   *tunnelClient
	incoming chan []byte
	closed   chan struct{}
	closeOnce sync.Once
	readBuf  []byte
}

type streamOpenPayload struct {
	TargetAddr        string `json:"targetAddr"`
	SourceIP          string `json:"sourceIp"`
	SourcePort        int    `json:"sourcePort"`
	RealIPSource      string `json:"realIpSource"`
	RuleID            int    `json:"ruleId"`
	RuleName          string `json:"ruleName"`
	Protocol          string `json:"protocol"`
	ProxyProtocol     string `json:"proxyProtocol"`
	ProxyProtocolMode string `json:"proxyProtocolMode"`
}

type dummyAddr string

const (
	tunnelFrameOpen  byte = 1
	tunnelFrameData  byte = 2
	tunnelFrameClose byte = 3
	tunnelFramePing  byte = 4
)

func main() {
	cfg := loadConfig()
	client := &apiClient{
		baseURL: strings.TrimRight(cfg.ControlURL, "/"),
		token:   cfg.Token,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
	reporter := &connectionReporter{active: map[string]connectionReport{}}
	tunnel := &tunnelClient{cfg: cfg, reporter: reporter, streams: map[uint64]*tunnelStream{}}
	manager := &relayManager{cfg: cfg, reporter: reporter, tunnel: tunnel, relays: map[int]*runningRelay{}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if _, err := client.register(ctx, cfg, reporter.totalActive()); err != nil {
		log.Printf("agent register failed: %v", err)
	} else {
		log.Printf("agent registered with control plane %s", cfg.ControlURL)
	}

	go heartbeatLoop(ctx, cfg, client, reporter)
	go tunnel.run(ctx)
	go configLoop(ctx, cfg, client, manager)
	go reportLoop(ctx, cfg, client, reporter)

	select {}
}

func loadConfig() config {
	cfg := config{}
	flag.StringVar(&cfg.ControlURL, "control", getenv("REPLEYPASS_CONTROL_URL", "http://localhost:8080"), "control plane base URL")
	flag.StringVar(&cfg.Token, "token", getenv("REPLEYPASS_AGENT_TOKEN", ""), "agent token")
	flag.StringVar(&cfg.AdvertiseAddress, "advertise", getenv("REPLEYPASS_AGENT_ADDRESS", ""), "address reported to control plane")
	flag.StringVar(&cfg.Region, "region", getenv("REPLEYPASS_AGENT_REGION", ""), "agent region")
	flag.StringVar(&cfg.Version, "version", getenv("REPLEYPASS_AGENT_VERSION", "edge-0.1.0"), "agent version")
	configSec := flag.Int("config-interval", atoiDefault(getenv("REPLEYPASS_AGENT_CONFIG_SEC", "20"), 20), "config sync interval seconds")
	heartbeatSec := flag.Int("heartbeat-interval", atoiDefault(getenv("REPLEYPASS_AGENT_HEARTBEAT_SEC", "30"), 30), "heartbeat interval seconds")
	reportSec := flag.Int("report-interval", atoiDefault(getenv("REPLEYPASS_AGENT_REPORT_SEC", "15"), 15), "connection report interval seconds")
	flag.Parse()

	if cfg.Token == "" {
		log.Fatal("agent token is required: set -token or REPLEYPASS_AGENT_TOKEN")
	}
	if err := validateControlURL(cfg.ControlURL); err != nil {
		log.Fatalf("invalid control URL: %v", err)
	}
	cfg.ConfigInterval = time.Duration(maxInt(*configSec, 5)) * time.Second
	cfg.HeartbeatInterval = time.Duration(maxInt(*heartbeatSec, 5)) * time.Second
	cfg.ReportInterval = time.Duration(maxInt(*reportSec, 5)) * time.Second
	return cfg
}

func heartbeatLoop(ctx context.Context, cfg config, client *apiClient, reporter *connectionReporter) {
	ticker := time.NewTicker(cfg.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := client.heartbeat(ctx, cfg, reporter.totalActive()); err != nil {
				log.Printf("heartbeat failed: %v", err)
			}
		}
	}
}

func configLoop(ctx context.Context, cfg config, client *apiClient, manager *relayManager) {
	ticker := time.NewTicker(cfg.ConfigInterval)
	defer ticker.Stop()
	for {
		envelope, err := client.config(ctx)
		if err != nil {
			log.Printf("config sync failed: %v", err)
		} else if err := manager.apply(ctx, envelope); err != nil {
			log.Printf("apply config failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func reportLoop(ctx context.Context, cfg config, client *apiClient, reporter *connectionReporter) {
	ticker := time.NewTicker(cfg.ReportInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reports := reporter.snapshot()
			if len(reports) == 0 {
				continue
			}
			if err := client.reportConnections(ctx, reports); err != nil {
				log.Printf("connection report failed: %v", err)
			}
		}
	}
}

func (t *tunnelClient) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := t.connectAndServe(ctx); err != nil {
			log.Printf("tunnel disconnected: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func (t *tunnelClient) connectAndServe(ctx context.Context) error {
	conn, err := dialTunnel(ctx, t.cfg.ControlURL, t.cfg.Token)
	if err != nil {
		return err
	}
	t.setConn(conn)
	defer func() {
		t.setConn(nil)
		_ = conn.Close()
		t.closeAllStreams()
	}()
	log.Printf("agent tunnel connected")
	for {
		frame, err := readTunnelFrame(conn)
		if err != nil {
			return err
		}
		switch frame.Type {
		case tunnelFrameOpen:
			go t.handleInboundOpen(ctx, frame)
		case tunnelFrameData:
			t.deliver(frame)
		case tunnelFrameClose:
			t.closeStream(frame.StreamID)
		case tunnelFramePing:
			_ = t.writeFrame(tunnelFrame{Type: tunnelFramePing, StreamID: frame.StreamID, TargetDeviceID: frame.SourceDeviceID})
		}
	}
}

func dialTunnel(ctx context.Context, controlURL string, token string) (net.Conn, error) {
	parsed, err := url.Parse(controlURL)
	if err != nil {
		return nil, err
	}
	host := parsed.Host
	if !strings.Contains(host, ":") {
		if parsed.Scheme == "https" {
			host = net.JoinHostPort(host, "443")
		} else {
			host = net.JoinHostPort(host, "80")
		}
	}
	dialer := net.Dialer{Timeout: 15 * time.Second}
	var conn net.Conn
	if parsed.Scheme == "https" {
		conn, err = tlsDialWithContext(ctx, &dialer, host, parsed.Hostname())
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", host)
	}
	if err != nil {
		return nil, err
	}
	path := strings.TrimRight(parsed.Path, "/") + "/api/v1/agent/tunnel"
	if parsed.Path == "" || parsed.Path == "/" {
		path = "/api/v1/agent/tunnel"
	}
	request := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nAuthorization: Bearer %s\r\nConnection: Upgrade\r\nUpgrade: repleypass-tunnel\r\n\r\n", path, parsed.Host, token)
	if _, err := conn.Write([]byte(request)); err != nil {
		_ = conn.Close()
		return nil, err
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		_ = conn.Close()
		return nil, fmt.Errorf("tunnel upgrade failed: %s", resp.Status)
	}
	return conn, nil
}

func tlsDialWithContext(ctx context.Context, dialer *net.Dialer, address string, serverName string) (net.Conn, error) {
	raw, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	conn := tls.Client(raw, &tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12})
	if err := conn.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return nil, err
	}
	return conn, nil
}

func (t *tunnelClient) setConn(conn net.Conn) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.conn = conn
}

func (t *tunnelClient) openStream(ctx context.Context, peerID int, payload streamOpenPayload) (*tunnelStream, error) {
	if peerID <= 0 {
		return nil, errors.New("missing tunnel peer")
	}
	if payload.TargetAddr == "" {
		return nil, errors.New("missing tunnel target")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	stream := &tunnelStream{
		id:       atomic.AddUint64(&t.nextID, 1),
		peerID:   peerID,
		client:   t,
		incoming: make(chan []byte, 64),
		closed:   make(chan struct{}),
	}
	t.addStream(stream)
	if err := t.writeFrame(tunnelFrame{Type: tunnelFrameOpen, StreamID: stream.id, TargetDeviceID: uint32(peerID), Payload: encoded}); err != nil {
		t.removeStream(stream.id)
		return nil, err
	}
	return stream, nil
}

func (t *tunnelClient) handleInboundOpen(ctx context.Context, frame tunnelFrame) {
	payload, err := decodeStreamOpenPayload(frame.Payload)
	if err != nil {
		_ = t.writeFrame(tunnelFrame{Type: tunnelFrameClose, StreamID: frame.StreamID, TargetDeviceID: frame.SourceDeviceID, Payload: []byte(err.Error())})
		return
	}
	if payload.TargetAddr == "" {
		_ = t.writeFrame(tunnelFrame{Type: tunnelFrameClose, StreamID: frame.StreamID, TargetDeviceID: frame.SourceDeviceID, Payload: []byte("missing target address")})
		return
	}
	stream := &tunnelStream{
		id:       frame.StreamID,
		peerID:   int(frame.SourceDeviceID),
		client:   t,
		incoming: make(chan []byte, 64),
		closed:   make(chan struct{}),
	}
	t.addStream(stream)
	defer t.removeStream(stream.id)
	dialer := net.Dialer{Timeout: 10 * time.Second}
	target, err := dialer.DialContext(ctx, "tcp", payload.TargetAddr)
	if err != nil {
		_ = t.writeFrame(tunnelFrame{Type: tunnelFrameClose, StreamID: frame.StreamID, TargetDeviceID: frame.SourceDeviceID, Payload: []byte(err.Error())})
		return
	}
	defer target.Close()
	if err := writeProxyHeaderIfNeeded(target, payload); err != nil {
		log.Printf("send tunnel proxy protocol failed stream=%d target=%s: %v", frame.StreamID, payload.TargetAddr, err)
		return
	}
	if payload.SourceIP != "" {
		log.Printf("reverse tunnel stream=%d source=%s:%d target=%s via=%s", frame.StreamID, payload.SourceIP, payload.SourcePort, payload.TargetAddr, firstNonEmpty(payload.RealIPSource, "reverse_tunnel"))
	}
	bridgeIO(target, stream)
}

func decodeStreamOpenPayload(data []byte) (streamOpenPayload, error) {
	var payload streamOpenPayload
	if len(data) == 0 {
		return payload, errors.New("empty tunnel open payload")
	}
	if data[0] == '{' {
		if err := json.Unmarshal(data, &payload); err != nil {
			return payload, err
		}
		return payload, nil
	}
	payload.TargetAddr = strings.TrimSpace(string(data))
	return payload, nil
}

func (t *tunnelClient) addStream(stream *tunnelStream) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.streams[stream.id] = stream
}

func (t *tunnelClient) removeStream(id uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.streams, id)
}

func (t *tunnelClient) closeStream(id uint64) {
	t.mu.RLock()
	stream := t.streams[id]
	t.mu.RUnlock()
	if stream != nil {
		stream.closeLocal()
	}
}

func (t *tunnelClient) closeAllStreams() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, stream := range t.streams {
		stream.closeLocal()
	}
	t.streams = map[uint64]*tunnelStream{}
}

func (t *tunnelClient) deliver(frame tunnelFrame) {
	t.mu.RLock()
	stream := t.streams[frame.StreamID]
	t.mu.RUnlock()
	if stream == nil {
		return
	}
	select {
	case stream.incoming <- frame.Payload:
	case <-stream.closed:
	}
}

func (t *tunnelClient) writeFrame(frame tunnelFrame) error {
	t.mu.RLock()
	conn := t.conn
	t.mu.RUnlock()
	if conn == nil {
		return errors.New("tunnel offline")
	}
	t.writerMu.Lock()
	defer t.writerMu.Unlock()
	return writeTunnelFrame(conn, frame)
}

func (c *apiClient) register(ctx context.Context, cfg config, connections int) (controlEnvelope, error) {
	payload := heartbeatPayload{
		Status:          "online",
		Address:         cfg.AdvertiseAddress,
		Region:          cfg.Region,
		Version:         cfg.Version,
		Load:            "0%",
		ConnectionCount: connections,
		InboundTraffic:  "0.00 GiB",
		OutboundTraffic: "0.00 GiB",
	}
	var envelope controlEnvelope
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/agent/register", payload, &envelope)
	return envelope, err
}

func (c *apiClient) heartbeat(ctx context.Context, cfg config, connections int) (controlEnvelope, error) {
	payload := heartbeatPayload{
		Status:          "online",
		Address:         cfg.AdvertiseAddress,
		Region:          cfg.Region,
		Version:         cfg.Version,
		Load:            "0%",
		ConnectionCount: connections,
		InboundTraffic:  "0.00 GiB",
		OutboundTraffic: "0.00 GiB",
	}
	var envelope controlEnvelope
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/agent/heartbeat", payload, &envelope)
	return envelope, err
}

func (c *apiClient) config(ctx context.Context) (controlEnvelope, error) {
	var envelope controlEnvelope
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/agent/config", nil, &envelope)
	return envelope, err
}

func (c *apiClient) reportConnections(ctx context.Context, reports []connectionReport) error {
	payload := map[string]interface{}{"connections": reports}
	return c.doJSON(ctx, http.MethodPost, "/api/v1/agent/connections", payload, nil)
}

func (c *apiClient) doJSON(ctx context.Context, method string, path string, payload interface{}, out interface{}) error {
	var body io.Reader
	if payload != nil {
		buf, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var api apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&api); err != nil {
		return err
	}
	if resp.StatusCode >= 400 || !api.Success {
		if api.Message == "" {
			api.Message = resp.Status
		}
		return errors.New(api.Message)
	}
	if out != nil && len(api.Data) > 0 {
		return json.Unmarshal(api.Data, out)
	}
	return nil
}

func (m *relayManager) apply(parent context.Context, envelope controlEnvelope) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	desired := map[int]agentRule{}
	for _, item := range envelope.Rules {
		if item.Role != "entry" && item.Role != "entry_exit" {
			continue
		}
		if !item.Rule.Enabled {
			continue
		}
		desired[item.Rule.ID] = item
	}

	for id, relay := range m.relays {
		item, ok := desired[id]
		if !ok || relay.key != relayKey(item.Rule) {
			relay.cancel()
			delete(m.relays, id)
		}
	}
	for id, item := range desired {
		if _, ok := m.relays[id]; ok {
			continue
		}
		ctx, cancel := context.WithCancel(parent)
		relay := &runningRelay{key: relayKey(item.Rule), rule: item.Rule, cancel: cancel}
		if err := m.startRelay(ctx, item); err != nil {
			cancel()
			log.Printf("skip rule id=%d name=%s: %v", item.Rule.ID, item.Rule.Name, err)
			continue
		}
		m.relays[id] = relay
	}
	return nil
}

func relayKey(rule forwardRule) string {
	return strings.Join([]string{rule.Protocol, rule.ListenHost, rule.ListenPort, rule.TargetHost, rule.TargetPort, rule.ProxyProtocolMode}, "|")
}

func (m *relayManager) startRelay(ctx context.Context, item agentRule) error {
	rule := item.Rule
	protocol := normalizeProtocol(rule.Protocol)
	switch protocol {
	case "tcp", "tls", "tls-passthrough":
		return m.startTCPRelay(ctx, rule)
	case "udp":
		return m.startUDPRelay(ctx, rule)
	case "http", "http-connect", "connect":
		return m.startHTTPConnectProxy(ctx, rule)
	case "socks5", "socks":
		return m.startSOCKS5Proxy(ctx, rule)
	case "ws", "wss", "reverse", "reverse-tunnel":
		return m.startReverseTunnelRelay(ctx, item)
	default:
		return fmt.Errorf("unsupported protocol %q", rule.Protocol)
	}
}

func (m *relayManager) startReverseTunnelRelay(ctx context.Context, item agentRule) error {
	rule := item.Rule
	listenAddr, err := listenAddress(rule)
	if err != nil {
		return err
	}
	targetAddr, err := targetAddress(rule)
	if err != nil {
		return err
	}
	peerID := selectExitDevice(item.ExitDevices)
	if peerID == 0 {
		return fmt.Errorf("rule %d has no exit device candidate", rule.ID)
	}
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	log.Printf("reverse tunnel rule=%d listen=%s peer=%d target=%s", rule.ID, listenAddr, peerID, targetAddr)
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("reverse tunnel accept failed rule=%d: %v", rule.ID, err)
				continue
			}
			go m.handleReverseTunnelConn(ctx, rule, conn, peerID, targetAddr)
		}
	}()
	return nil
}

func selectExitDevice(devices []device) int {
	for _, item := range devices {
		if item.Enabled && item.Status == "online" {
			return item.ID
		}
	}
	for _, item := range devices {
		if item.Enabled {
			return item.ID
		}
	}
	return 0
}

func (m *relayManager) handleReverseTunnelConn(ctx context.Context, rule forwardRule, client net.Conn, peerID int, targetAddr string) {
	clientConn, report := prepareClientConn(rule, client)
	m.reporter.add(report)
	defer m.reporter.remove(report)
	defer clientConn.Close()

	stream, err := m.tunnel.openStream(ctx, peerID, streamOpenPayload{
		TargetAddr:        targetAddr,
		SourceIP:          report.SourceIP,
		SourcePort:        report.SourcePort,
		RealIPSource:      report.RealIPSource,
		RuleID:            rule.ID,
		RuleName:          rule.Name,
		Protocol:          rule.Protocol,
		ProxyProtocol:     rule.ProxyProtocol,
		ProxyProtocolMode: rule.ProxyProtocolMode,
	})
	if err != nil {
		log.Printf("open reverse tunnel failed rule=%d peer=%d: %v", rule.ID, peerID, err)
		return
	}
	defer stream.Close()
	bridgeIO(clientConn, stream)
}

func (m *relayManager) startTCPRelay(ctx context.Context, rule forwardRule) error {
	listenAddr, err := listenAddress(rule)
	if err != nil {
		return err
	}
	targetAddr, err := targetAddress(rule)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	log.Printf("tcp relay rule=%d listen=%s target=%s", rule.ID, listenAddr, targetAddr)
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("tcp accept failed rule=%d: %v", rule.ID, err)
				continue
			}
			go m.handleTCPConn(ctx, rule, conn, targetAddr)
		}
	}()
	return nil
}

func (m *relayManager) handleTCPConn(ctx context.Context, rule forwardRule, client net.Conn, targetAddr string) {
	clientConn, report := prepareClientConn(rule, client)
	m.reporter.add(report)
	defer m.reporter.remove(report)
	defer clientConn.Close()

	dialer := net.Dialer{Timeout: 10 * time.Second}
	target, err := dialer.DialContext(ctx, "tcp", targetAddr)
	if err != nil {
		log.Printf("tcp dial failed rule=%d target=%s: %v", rule.ID, targetAddr, err)
		return
	}
	defer target.Close()
	if err := writeProxyHeaderIfNeeded(target, payloadFromRule(rule, report, targetAddr)); err != nil {
		log.Printf("send proxy protocol failed rule=%d target=%s: %v", rule.ID, targetAddr, err)
		return
	}
	bridge(clientConn, target)
}

func (m *relayManager) startUDPRelay(ctx context.Context, rule forwardRule) error {
	listenAddr, err := listenAddress(rule)
	if err != nil {
		return err
	}
	targetAddr, err := targetAddress(rule)
	if err != nil {
		return err
	}
	udpAddr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return err
	}
	targetUDP, err := net.ResolveUDPAddr("udp", targetAddr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	log.Printf("udp relay rule=%d listen=%s target=%s", rule.ID, listenAddr, targetAddr)
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()
	go func() {
		buf := make([]byte, 64*1024)
		for {
			n, clientAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("udp read failed rule=%d: %v", rule.ID, err)
				continue
			}
			report := reportFromAddr(rule, clientAddr, "connection_log")
			m.reporter.add(report)
			go func(packet []byte, source *net.UDPAddr) {
				defer m.reporter.remove(report)
				relayUDPDatagram(ctx, conn, targetUDP, source, packet)
			}(append([]byte(nil), buf[:n]...), clientAddr)
		}
	}()
	return nil
}

func relayUDPDatagram(ctx context.Context, listener *net.UDPConn, target *net.UDPAddr, source *net.UDPAddr, packet []byte) {
	conn, err := net.DialUDP("udp", nil, target)
	if err != nil {
		log.Printf("udp dial failed target=%s: %v", target.String(), err)
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write(packet); err != nil {
		log.Printf("udp write failed: %v", err)
		return
	}
	buf := make([]byte, 64*1024)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
		_, _ = listener.WriteToUDP(buf[:n], source)
	}
}

func (m *relayManager) startHTTPConnectProxy(ctx context.Context, rule forwardRule) error {
	listenAddr, err := listenAddress(rule)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	log.Printf("http connect relay rule=%d listen=%s", rule.ID, listenAddr)
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("http accept failed rule=%d: %v", rule.ID, err)
				continue
			}
			go m.handleHTTPConnect(ctx, rule, conn)
		}
	}()
	return nil
}

func (m *relayManager) handleHTTPConnect(ctx context.Context, rule forwardRule, client net.Conn) {
	report := reportFromConn(rule, client.RemoteAddr(), "connection_log")
	m.reporter.add(report)
	defer m.reporter.remove(report)
	defer client.Close()

	reader := bufio.NewReader(client)
	req, err := http.ReadRequest(reader)
	if err != nil {
		return
	}
	if req.Method != http.MethodConnect {
		_, _ = client.Write([]byte("HTTP/1.1 405 Method Not Allowed\r\n\r\n"))
		return
	}
	targetAddr := req.Host
	if _, _, err := net.SplitHostPort(targetAddr); err != nil {
		targetAddr = net.JoinHostPort(targetAddr, "443")
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	target, err := dialer.DialContext(ctx, "tcp", targetAddr)
	if err != nil {
		_, _ = client.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer target.Close()
	_, _ = client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	bridge(&bufferedConn{Conn: client, reader: reader}, target)
}

func (m *relayManager) startSOCKS5Proxy(ctx context.Context, rule forwardRule) error {
	listenAddr, err := listenAddress(rule)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	log.Printf("socks5 relay rule=%d listen=%s", rule.ID, listenAddr)
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("socks5 accept failed rule=%d: %v", rule.ID, err)
				continue
			}
			go m.handleSOCKS5(ctx, rule, conn)
		}
	}()
	return nil
}

func (m *relayManager) handleSOCKS5(ctx context.Context, rule forwardRule, client net.Conn) {
	report := reportFromConn(rule, client.RemoteAddr(), "connection_log")
	m.reporter.add(report)
	defer m.reporter.remove(report)
	defer client.Close()

	reader := bufio.NewReader(client)
	if err := socks5Handshake(reader, client); err != nil {
		return
	}
	targetAddr, err := socks5Target(reader)
	if err != nil {
		_, _ = client.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	target, err := dialer.DialContext(ctx, "tcp", targetAddr)
	if err != nil {
		_, _ = client.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer target.Close()
	_, _ = client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	bridge(&bufferedConn{Conn: client, reader: reader}, target)
}

func socks5Handshake(reader *bufio.Reader, writer io.Writer) error {
	header, err := reader.Peek(2)
	if err != nil {
		return err
	}
	if header[0] != 0x05 {
		return errors.New("not socks5")
	}
	methodLen := int(header[1])
	if _, err := reader.Discard(2 + methodLen); err != nil {
		return err
	}
	_, err = writer.Write([]byte{0x05, 0x00})
	return err
}

func socks5Target(reader *bufio.Reader) (string, error) {
	head := make([]byte, 4)
	if _, err := io.ReadFull(reader, head); err != nil {
		return "", err
	}
	if head[0] != 0x05 || head[1] != 0x01 {
		return "", errors.New("unsupported socks5 command")
	}
	var host string
	switch head[3] {
	case 0x01:
		buf := make([]byte, 4)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return "", err
		}
		host = net.IP(buf).String()
	case 0x03:
		length, err := reader.ReadByte()
		if err != nil {
			return "", err
		}
		buf := make([]byte, int(length))
		if _, err := io.ReadFull(reader, buf); err != nil {
			return "", err
		}
		host = string(buf)
	case 0x04:
		buf := make([]byte, 16)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return "", err
		}
		host = net.IP(buf).String()
	default:
		return "", errors.New("unsupported socks5 address type")
	}
	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(reader, portBuf); err != nil {
		return "", err
	}
	return net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(portBuf)))), nil
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	if c.reader != nil && c.reader.Buffered() > 0 {
		return c.reader.Read(p)
	}
	return c.Conn.Read(p)
}

func bridge(a net.Conn, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(a, b)
		_ = a.SetDeadline(time.Now())
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(b, a)
		_ = b.SetDeadline(time.Now())
	}()
	wg.Wait()
}

func bridgeIO(a io.ReadWriteCloser, b io.ReadWriteCloser) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(a, b)
		_ = a.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(b, a)
		_ = b.Close()
	}()
	wg.Wait()
}

func (s *tunnelStream) Read(p []byte) (int, error) {
	for len(s.readBuf) == 0 {
		select {
		case data := <-s.incoming:
			if len(data) == 0 {
				continue
			}
			s.readBuf = data
		case <-s.closed:
			return 0, io.EOF
		}
	}
	n := copy(p, s.readBuf)
	s.readBuf = s.readBuf[n:]
	return n, nil
}

func (s *tunnelStream) Write(p []byte) (int, error) {
	select {
	case <-s.closed:
		return 0, io.ErrClosedPipe
	default:
	}
	data := append([]byte(nil), p...)
	if err := s.client.writeFrame(tunnelFrame{Type: tunnelFrameData, StreamID: s.id, TargetDeviceID: uint32(s.peerID), Payload: data}); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *tunnelStream) Close() error {
	s.closeOnce.Do(func() {
		_ = s.client.writeFrame(tunnelFrame{Type: tunnelFrameClose, StreamID: s.id, TargetDeviceID: uint32(s.peerID)})
		close(s.closed)
		s.client.removeStream(s.id)
	})
	return nil
}

func (s *tunnelStream) closeLocal() {
	s.closeOnce.Do(func() {
		close(s.closed)
		s.client.removeStream(s.id)
	})
}

func (a dummyAddr) Network() string {
	return "tunnel"
}

func (a dummyAddr) String() string {
	return string(a)
}

func readTunnelFrame(r io.Reader) (tunnelFrame, error) {
	var header [21]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return tunnelFrame{}, err
	}
	length := binary.BigEndian.Uint32(header[17:21])
	if length > 16*1024*1024 {
		return tunnelFrame{}, errors.New("tunnel frame too large")
	}
	payload := make([]byte, int(length))
	if length > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return tunnelFrame{}, err
		}
	}
	return tunnelFrame{
		Type:           header[0],
		StreamID:       binary.BigEndian.Uint64(header[1:9]),
		SourceDeviceID: binary.BigEndian.Uint32(header[9:13]),
		TargetDeviceID: binary.BigEndian.Uint32(header[13:17]),
		Payload:        payload,
	}, nil
}

func writeTunnelFrame(w io.Writer, frame tunnelFrame) error {
	var header [21]byte
	header[0] = frame.Type
	binary.BigEndian.PutUint64(header[1:9], frame.StreamID)
	binary.BigEndian.PutUint32(header[9:13], frame.SourceDeviceID)
	binary.BigEndian.PutUint32(header[13:17], frame.TargetDeviceID)
	binary.BigEndian.PutUint32(header[17:21], uint32(len(frame.Payload)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	if len(frame.Payload) > 0 {
		_, err := w.Write(frame.Payload)
		return err
	}
	return nil
}

func listenAddress(rule forwardRule) (string, error) {
	port := firstPort(rule.ListenPort)
	if port == "" {
		return "", fmt.Errorf("rule %d missing listen port", rule.ID)
	}
	return net.JoinHostPort(rule.ListenHost, port), nil
}

func targetAddress(rule forwardRule) (string, error) {
	host := strings.TrimSpace(rule.TargetHost)
	port := firstPort(rule.TargetPort)
	if host == "" {
		return "", fmt.Errorf("rule %d missing target host", rule.ID)
	}
	if port == "" {
		return "", fmt.Errorf("rule %d missing target port", rule.ID)
	}
	return net.JoinHostPort(host, port), nil
}

func firstPort(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, sep := range []string{"-", ",", " "} {
		if strings.Contains(value, sep) {
			value = strings.TrimSpace(strings.Split(value, sep)[0])
		}
	}
	return value
}

func normalizeProtocol(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	switch value {
	case "tls-入站", "tls":
		return "tls-passthrough"
	case "http-connect":
		return "http-connect"
	case "socks":
		return "socks5"
	default:
		return value
	}
}

func reportFromConn(rule forwardRule, addr net.Addr, source string) connectionReport {
	host, portText, _ := net.SplitHostPort(addr.String())
	port, _ := strconv.Atoi(portText)
	return reportFromParts(rule, host, port, source)
}

func prepareClientConn(rule forwardRule, conn net.Conn) (net.Conn, connectionReport) {
	if !shouldReceiveProxy(rule) {
		return conn, reportFromConn(rule, conn.RemoteAddr(), "connection_log")
	}
	reader := bufio.NewReader(conn)
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	info, ok, err := readProxyProtocol(reader)
	_ = conn.SetReadDeadline(time.Time{})
	if err != nil {
		log.Printf("proxy protocol parse failed rule=%d remote=%s: %v", rule.ID, conn.RemoteAddr().String(), err)
		return &bufferedConn{Conn: conn, reader: reader}, reportFromConn(rule, conn.RemoteAddr(), "connection_log")
	}
	if !ok {
		return &bufferedConn{Conn: conn, reader: reader}, reportFromConn(rule, conn.RemoteAddr(), "connection_log")
	}
	return &bufferedConn{Conn: conn, reader: reader}, reportFromParts(rule, info.SourceIP, info.SourcePort, info.Source)
}

type proxyInfo struct {
	Source     string
	SourceIP   string
	SourcePort int
}

func readProxyProtocol(reader *bufio.Reader) (proxyInfo, bool, error) {
	if header, err := reader.Peek(5); err == nil && string(header) == "PROXY" {
		return readProxyProtocolV1(reader)
	}
	if header, err := reader.Peek(12); err == nil && bytes.Equal(header, proxyV2Signature()) {
		return readProxyProtocolV2(reader)
	}
	return proxyInfo{}, false, nil
}

func readProxyProtocolV1(reader *bufio.Reader) (proxyInfo, bool, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return proxyInfo{}, false, err
	}
	line = strings.TrimRight(line, "\r\n")
	parts := strings.Fields(line)
	if len(parts) < 6 || parts[0] != "PROXY" {
		return proxyInfo{}, false, errors.New("invalid proxy protocol v1 header")
	}
	port, err := strconv.Atoi(parts[4])
	if err != nil {
		return proxyInfo{}, false, err
	}
	return proxyInfo{Source: "proxy_protocol_v1", SourceIP: parts[2], SourcePort: port}, true, nil
}

func readProxyProtocolV2(reader *bufio.Reader) (proxyInfo, bool, error) {
	var header [16]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return proxyInfo{}, false, err
	}
	length := int(binary.BigEndian.Uint16(header[14:16]))
	addr := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(reader, addr); err != nil {
			return proxyInfo{}, false, err
		}
	}
	if header[12]>>4 != 0x2 {
		return proxyInfo{}, false, errors.New("invalid proxy protocol v2 version")
	}
	if header[12]&0x0f != 0x1 {
		return proxyInfo{}, false, nil
	}
	switch header[13] {
	case 0x11, 0x12:
		if len(addr) < 12 {
			return proxyInfo{}, false, errors.New("short proxy protocol v2 ipv4 address")
		}
		return proxyInfo{Source: "proxy_protocol_v2", SourceIP: net.IP(addr[0:4]).String(), SourcePort: int(binary.BigEndian.Uint16(addr[8:10]))}, true, nil
	case 0x21, 0x22:
		if len(addr) < 36 {
			return proxyInfo{}, false, errors.New("short proxy protocol v2 ipv6 address")
		}
		return proxyInfo{Source: "proxy_protocol_v2", SourceIP: net.IP(addr[0:16]).String(), SourcePort: int(binary.BigEndian.Uint16(addr[32:34]))}, true, nil
	default:
		return proxyInfo{}, false, nil
	}
}

func proxyV2Signature() []byte {
	return []byte{0x0d, 0x0a, 0x0d, 0x0a, 0x00, 0x0d, 0x0a, 0x51, 0x55, 0x49, 0x54, 0x0a}
}

func reportFromAddr(rule forwardRule, addr *net.UDPAddr, source string) connectionReport {
	return reportFromParts(rule, addr.IP.String(), addr.Port, source)
}

func reportFromParts(rule forwardRule, sourceIP string, sourcePort int, source string) connectionReport {
	return connectionReport{
		SourceIP:        sourceIP,
		SourcePort:      sourcePort,
		RuleID:          rule.ID,
		RuleName:        rule.Name,
		Protocol:        rule.Protocol,
		RealIPSource:    source,
		ConnectionCount: 1,
	}
}

func payloadFromRule(rule forwardRule, report connectionReport, targetAddr string) streamOpenPayload {
	return streamOpenPayload{
		TargetAddr:        targetAddr,
		SourceIP:          report.SourceIP,
		SourcePort:        report.SourcePort,
		RealIPSource:      report.RealIPSource,
		RuleID:            rule.ID,
		RuleName:          rule.Name,
		Protocol:          rule.Protocol,
		ProxyProtocol:     rule.ProxyProtocol,
		ProxyProtocolMode: rule.ProxyProtocolMode,
	}
}

func shouldReceiveProxy(rule forwardRule) bool {
	mode := strings.ToLower(strings.TrimSpace(rule.ProxyProtocolMode))
	proxy := strings.ToLower(strings.TrimSpace(rule.ProxyProtocol))
	return mode == "receive" || strings.Contains(mode, "接收") || strings.Contains(proxy, "接收")
}

func shouldSendProxy(protocol string, mode string) bool {
	mode = strings.ToLower(strings.TrimSpace(mode))
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if mode != "send" && !strings.Contains(mode, "发送") {
		return false
	}
	return strings.Contains(protocol, "v1") || strings.Contains(protocol, "v2")
}

func writeProxyHeaderIfNeeded(w io.Writer, payload streamOpenPayload) error {
	if !shouldSendProxy(payload.ProxyProtocol, payload.ProxyProtocolMode) || payload.SourceIP == "" {
		return nil
	}
	if strings.Contains(strings.ToLower(payload.ProxyProtocol), "v2") {
		return writeProxyProtocolV2(w, payload.SourceIP, payload.SourcePort)
	}
	return writeProxyProtocolV1(w, payload.SourceIP, payload.SourcePort)
}

func writeProxyProtocolV1(w io.Writer, sourceIP string, sourcePort int) error {
	family := "TCP4"
	if strings.Contains(sourceIP, ":") {
		family = "TCP6"
	}
	_, err := fmt.Fprintf(w, "PROXY %s %s %s %d %d\r\n", family, sourceIP, zeroIP(family), sourcePort, 0)
	return err
}

func writeProxyProtocolV2(w io.Writer, sourceIP string, sourcePort int) error {
	ip := net.ParseIP(sourceIP)
	if ip == nil {
		return writeProxyProtocolV1(w, sourceIP, sourcePort)
	}
	header := append([]byte{}, proxyV2Signature()...)
	header = append(header, 0x21)
	if ipv4 := ip.To4(); ipv4 != nil {
		header = append(header, 0x11, 0x00, 0x0c)
		header = append(header, ipv4...)
		header = append(header, []byte{0, 0, 0, 0}...)
		var ports [4]byte
		binary.BigEndian.PutUint16(ports[0:2], uint16(sourcePort))
		header = append(header, ports[:]...)
		_, err := w.Write(header)
		return err
	}
	ipv6 := ip.To16()
	header = append(header, 0x21, 0x00, 0x24)
	header = append(header, ipv6...)
	header = append(header, make([]byte, 16)...)
	var ports [4]byte
	binary.BigEndian.PutUint16(ports[0:2], uint16(sourcePort))
	header = append(header, ports[:]...)
	_, err := w.Write(header)
	return err
}

func zeroIP(family string) string {
	if family == "TCP6" {
		return net.IPv6zero.String()
	}
	return net.IPv4zero.String()
}

func (r *connectionReporter) add(report connectionReport) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := connectionKey(report)
	current := r.active[key]
	if current.SourceIP == "" {
		r.active[key] = report
		atomic.AddInt64(&r.total, 1)
		return
	}
	current.ConnectionCount++
	r.active[key] = current
	atomic.AddInt64(&r.total, 1)
}

func (r *connectionReporter) remove(report connectionReport) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := connectionKey(report)
	current := r.active[key]
	if current.ConnectionCount <= 1 {
		delete(r.active, key)
	} else {
		current.ConnectionCount--
		r.active[key] = current
	}
	atomic.AddInt64(&r.total, -1)
}

func (r *connectionReporter) snapshot() []connectionReport {
	r.mu.Lock()
	defer r.mu.Unlock()
	reports := make([]connectionReport, 0, len(r.active))
	for _, report := range r.active {
		reports = append(reports, report)
	}
	return reports
}

func (r *connectionReporter) totalActive() int {
	value := atomic.LoadInt64(&r.total)
	if value < 0 {
		return 0
	}
	return int(value)
}

func connectionKey(report connectionReport) string {
	return fmt.Sprintf("%s:%d:%d", report.SourceIP, report.SourcePort, report.RuleID)
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func atoiDefault(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func validateControlURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("control URL must include scheme and host")
	}
	return nil
}
