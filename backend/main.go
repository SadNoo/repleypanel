package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const sessionCookieName = "rp_session"
const defaultAgentTokenTTL = 30 * 24 * time.Hour

type response struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data"`
	Message   string      `json:"message"`
	RequestID string      `json:"requestId"`
}

type pageData struct {
	Items    interface{} `json:"items"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
}

type user struct {
	ID               int    `json:"id"`
	UID              int    `json:"uid"`
	Username         string `json:"username"`
	Role             string `json:"role"`
	UserType         string `json:"userType"`
	UserGroup        string `json:"userGroup"`
	Plan             string `json:"plan"`
	PlanExpire       string `json:"planExpire"`
	TrafficUsed      string `json:"trafficUsed"`
	TrafficTotal     string `json:"trafficTotal"`
	MaxRules         int    `json:"maxRules"`
	RateLimit        string `json:"rateLimit"`
	ConnectionLimit  int    `json:"connectionLimit"`
	WalletBalance    string `json:"walletBalance"`
	TelegramLinked   bool   `json:"telegramLinked"`
	AutoRenewEnabled bool   `json:"autoRenewEnabled"`
	Remark           string `json:"remark"`
}

type forwardRule struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	Entry              string `json:"entry"`
	Exit               string `json:"exit"`
	EntryGroupID       int    `json:"entryGroupId"`
	EntryGroupName     string `json:"entryGroupName"`
	ListenHost         string `json:"listenHost"`
	ListenPort         string `json:"listenPort"`
	ExitGroupID        int    `json:"exitGroupId"`
	ExitGroupName      string `json:"exitGroupName"`
	TargetHost         string `json:"targetHost"`
	TargetPort         string `json:"targetPort"`
	Strategy           string `json:"strategy"`
	UsedTraffic        string `json:"usedTraffic"`
	TodayTraffic       string `json:"todayTraffic"`
	Status             string `json:"status"`
	SyncStatus         string `json:"syncStatus"`
	Group              string `json:"group"`
	Protocol           string `json:"protocol"`
	Proxy              string `json:"proxyProtocol"`
	ProxyProtocolMode  string `json:"proxyProtocolMode"`
	Owner              string `json:"owner"`
	CurrentConnections int    `json:"currentConnections"`
	ErrorCount         int    `json:"errorCount"`
	LastHitAt          string `json:"lastHitAt"`
	Enabled            bool   `json:"enabled"`
	Remark             string `json:"remark"`
}

type auditLog struct {
	ID         int    `json:"id"`
	Actor      string `json:"actor"`
	Action     string `json:"action"`
	Resource   string `json:"resource"`
	ResourceID int    `json:"resourceId"`
	Message    string `json:"message"`
	CreatedAt  string `json:"createdAt"`
}

type onlineIP struct {
	ID              int    `json:"id"`
	SourceIP        string `json:"sourceIp"`
	SourcePort      int    `json:"sourcePort"`
	RuleID          int    `json:"ruleId"`
	RuleName        string `json:"ruleName"`
	EntryDeviceID   int    `json:"entryDeviceId"`
	EntryDeviceName string `json:"entryDeviceName"`
	EntryGroupName  string `json:"entryGroupName"`
	Protocol        string `json:"protocol"`
	RealIPSource    string `json:"realIpSource"`
	ConnectionCount int    `json:"connectionCount"`
	FirstSeen       string `json:"firstSeen"`
	LastActiveAt    string `json:"lastActiveAt"`
	Status          string `json:"status"`
	Country         string `json:"country"`
	UserAgent       string `json:"userAgent"`
	Remark          string `json:"remark"`
}

type healthCheck struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	TargetType    string `json:"targetType"`
	TargetID      int    `json:"targetId"`
	TargetName    string `json:"targetName"`
	Protocol      string `json:"protocol"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Path          string `json:"path"`
	IntervalSec   int    `json:"intervalSec"`
	TimeoutSec    int    `json:"timeoutSec"`
	Status        string `json:"status"`
	LastLatencyMs int    `json:"lastLatencyMs"`
	LastError     string `json:"lastError"`
	LastCheckedAt string `json:"lastCheckedAt"`
	Enabled       bool   `json:"enabled"`
	Remark        string `json:"remark"`
}

type healthCheckResult struct {
	ID            int    `json:"id"`
	CheckID       int    `json:"checkId"`
	Status        string `json:"status"`
	LatencyMs     int    `json:"latencyMs"`
	FailureReason string `json:"failureReason"`
	CheckedAt     string `json:"checkedAt"`
}

type deviceGroup struct {
	ID             int     `json:"id"`
	Sort           int     `json:"sort"`
	Name           string  `json:"name"`
	UserGroupID    string  `json:"userGroupId"`
	Type           string  `json:"type"`
	DisplayAddress string  `json:"displayAddress"`
	Multiplier     float64 `json:"multiplier"`
	UsedTraffic    string  `json:"usedTraffic"`
	OnlineDevices  int     `json:"onlineDevices"`
	Remark         string  `json:"remark"`
}

type device struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	GroupID             int    `json:"groupId"`
	GroupName           string `json:"groupName"`
	Type                string `json:"type"`
	Status              string `json:"status"`
	Address             string `json:"address"`
	Region              string `json:"region"`
	Version             string `json:"version"`
	Load                string `json:"load"`
	LatencyMs           int    `json:"latencyMs"`
	ConnectionCount     int    `json:"connectionCount"`
	InboundTraffic      string `json:"inboundTraffic"`
	OutboundTraffic     string `json:"outboundTraffic"`
	LastHeartbeat       string `json:"lastHeartbeat"`
	LastSeen            string `json:"lastSeen"`
	Enabled             bool   `json:"enabled"`
	AgentRegisteredAt   string `json:"agentRegisteredAt"`
	AgentTokenExpiresAt string `json:"agentTokenExpiresAt"`
	AgentTokenRotatedAt string `json:"agentTokenRotatedAt"`
	ConfigVersion       int    `json:"configVersion"`
	Remark              string `json:"remark"`
}

type agentConnectionReport struct {
	SourceIP        string `json:"sourceIp"`
	SourcePort      int    `json:"sourcePort"`
	RuleID          int    `json:"ruleId"`
	RuleName        string `json:"ruleName"`
	Protocol        string `json:"protocol"`
	RealIPSource    string `json:"realIpSource"`
	ConnectionCount int    `json:"connectionCount"`
	Country         string `json:"country"`
	UserAgent       string `json:"userAgent"`
	Remark          string `json:"remark"`
}

type agentRulePayload struct {
	Role        string            `json:"role"`
	Mode        string            `json:"mode"`
	Rule        forwardRule       `json:"rule"`
	Entry       agentEntryConfig  `json:"entry"`
	Exit        agentExitConfig   `json:"exit"`
	Tunnel      agentTunnelConfig `json:"tunnel"`
	ExitDevices []device          `json:"exitDevices"`
}

type agentEntryConfig struct {
	Enabled           bool   `json:"enabled"`
	ListenHost        string `json:"listenHost"`
	ListenPort        string `json:"listenPort"`
	ListenAddr        string `json:"listenAddr"`
	Protocol          string `json:"protocol"`
	ProxyProtocol     string `json:"proxyProtocol"`
	ProxyProtocolMode string `json:"proxyProtocolMode"`
}

type agentExitConfig struct {
	Enabled           bool   `json:"enabled"`
	TargetHost        string `json:"targetHost"`
	TargetPort        string `json:"targetPort"`
	TargetAddr        string `json:"targetAddr"`
	ProxyProtocol     string `json:"proxyProtocol"`
	ProxyProtocolMode string `json:"proxyProtocolMode"`
}

type agentTunnelConfig struct {
	Enabled       bool     `json:"enabled"`
	PeerPolicy    string   `json:"peerPolicy"`
	PeerDeviceIDs []int    `json:"peerDeviceIds"`
	ExitDevices   []device `json:"exitDevices"`
}

type shopPlan struct {
	ID          int    `json:"id"`
	Sort        int    `json:"sort"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	UserGroup   string `json:"userGroup"`
	Traffic     string `json:"traffic"`
	RuleLimit   int    `json:"ruleLimit"`
	Price       string `json:"price"`
	Hidden      bool   `json:"hidden"`
	Description string `json:"description"`
}

type order struct {
	ID         string `json:"id"`
	User       string `json:"user"`
	CreatedAt  string `json:"createdAt"`
	PaidAt     string `json:"paidAt"`
	Info       string `json:"info"`
	Amount     string `json:"amount"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Commission string `json:"commission,omitempty"`
}

type redeemCode struct {
	ID       int    `json:"id"`
	Code     string `json:"code"`
	Plan     string `json:"plan"`
	Discount string `json:"discount"`
	Remain   int    `json:"remain"`
}

type userGroup struct {
	ID        int    `json:"id"`
	Sort      int    `json:"sort"`
	Name      string `json:"name"`
	UserCount int    `json:"userCount"`
}

type server struct {
	db            *sql.DB
	startedAt     time.Time
	dbPath        string
	healthCheckMu sync.Mutex
	tunnels       *tunnelHub
}

type tunnelHub struct {
	mu       sync.RWMutex
	conns    map[int]*tunnelPeer
	routes   map[string]int
	validate tunnelValidator
}

type tunnelPeer struct {
	deviceID int
	conn     net.Conn
	mu       sync.Mutex
}

type tunnelFrame struct {
	Type           byte
	StreamID       uint64
	SourceDeviceID uint32
	TargetDeviceID uint32
	Payload        []byte
}

type tunnelOpenPayload struct {
	TargetAddr   string `json:"targetAddr"`
	SourceIP     string `json:"sourceIp"`
	SourcePort   int    `json:"sourcePort"`
	RealIPSource string `json:"realIpSource"`
	RuleID       int    `json:"ruleId"`
	RuleName     string `json:"ruleName"`
	Protocol     string `json:"protocol"`
}

type tunnelValidator func(sourceDeviceID int, frame tunnelFrame) error

const (
	tunnelFrameOpen  byte = 1
	tunnelFrameData  byte = 2
	tunnelFrameClose byte = 3
	tunnelFramePing  byte = 4
)

func main() {
	addr := getenv("REPLEYPASS_ADDR", ":8080")
	dbPath := getenv("REPLEYPASS_DB", "./repleypass.db")
	db, err := openStore(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	s := &server{db: db, startedAt: time.Now(), dbPath: dbPath}
	s.tunnels = newTunnelHub(s.validateTunnelOpen)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/api/v1/", s.api)

	go s.startHealthCheckScheduler(context.Background())

	log.Printf("RepleyPass API listening on %s db=%s", addr, dbPath)
	if err := http.ListenAndServe(addr, logRequests(mux)); err != nil {
		log.Fatal(err)
	}
}

func openStore(dbPath string) (*sql.DB, error) {
	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return nil, err
		}
	}
	dsn := dbPath + "?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := seed(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			user_type TEXT NOT NULL DEFAULT '普通用户',
			user_group TEXT NOT NULL DEFAULT '#1',
			plan TEXT NOT NULL DEFAULT '#55',
			plan_expire TEXT NOT NULL DEFAULT '',
			traffic_used TEXT NOT NULL DEFAULT '0.00 GiB',
			traffic_total TEXT NOT NULL DEFAULT '0.00 GiB',
			max_rules INTEGER NOT NULL DEFAULT 0,
			rate_limit TEXT NOT NULL DEFAULT '',
			connection_limit INTEGER NOT NULL DEFAULT 0,
			wallet_balance TEXT NOT NULL DEFAULT '0 元',
			telegram_linked INTEGER NOT NULL DEFAULT 0,
			auto_renew_enabled INTEGER NOT NULL DEFAULT 0,
			remark TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS forward_rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			entry TEXT NOT NULL,
			exit TEXT NOT NULL,
			entry_group_id INTEGER NOT NULL DEFAULT 0,
			entry_group_name TEXT NOT NULL DEFAULT '',
			listen_host TEXT NOT NULL DEFAULT '',
			listen_port TEXT NOT NULL DEFAULT '',
			exit_group_id INTEGER NOT NULL DEFAULT 0,
			exit_group_name TEXT NOT NULL DEFAULT '',
			target_host TEXT NOT NULL DEFAULT '',
			target_port TEXT NOT NULL DEFAULT '',
			strategy TEXT NOT NULL DEFAULT 'fallback',
			used_traffic TEXT NOT NULL DEFAULT '0.00 GiB',
			today_traffic TEXT NOT NULL DEFAULT '0.00 GiB',
			status TEXT NOT NULL DEFAULT '未同步',
			sync_status TEXT NOT NULL DEFAULT '未同步',
			rule_group TEXT NOT NULL DEFAULT '未分组',
			protocol TEXT NOT NULL DEFAULT 'TCP',
			proxy_protocol TEXT NOT NULL DEFAULT '关闭',
			proxy_protocol_mode TEXT NOT NULL DEFAULT 'send',
			owner TEXT NOT NULL DEFAULT 'admin',
			current_connections INTEGER NOT NULL DEFAULT 0,
			error_count INTEGER NOT NULL DEFAULT 0,
			last_hit_at TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			remark TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS device_groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			sort INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			user_group_id TEXT NOT NULL DEFAULT '#1',
			type TEXT NOT NULL DEFAULT '出口',
			display_address TEXT NOT NULL DEFAULT '-',
			multiplier REAL NOT NULL DEFAULT 1,
			used_traffic TEXT NOT NULL DEFAULT '0.00 GiB',
			online_devices INTEGER NOT NULL DEFAULT 0,
			remark TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			group_id INTEGER NOT NULL DEFAULT 0,
			group_name TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT '出口',
			status TEXT NOT NULL DEFAULT 'offline',
			address TEXT NOT NULL DEFAULT '',
			region TEXT NOT NULL DEFAULT '',
			version TEXT NOT NULL DEFAULT '',
			load TEXT NOT NULL DEFAULT '0%',
			latency_ms INTEGER NOT NULL DEFAULT 0,
			connection_count INTEGER NOT NULL DEFAULT 0,
			inbound_traffic TEXT NOT NULL DEFAULT '0.00 GiB',
			outbound_traffic TEXT NOT NULL DEFAULT '0.00 GiB',
			last_heartbeat TEXT NOT NULL DEFAULT '',
			last_seen TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			agent_token_hash TEXT NOT NULL DEFAULT '',
			agent_registered_at TEXT NOT NULL DEFAULT '',
			agent_token_expires_at TEXT NOT NULL DEFAULT '',
			agent_token_rotated_at TEXT NOT NULL DEFAULT '',
			config_version INTEGER NOT NULL DEFAULT 1,
			remark TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS online_ips (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_ip TEXT NOT NULL,
			source_port INTEGER NOT NULL DEFAULT 0,
			rule_id INTEGER NOT NULL DEFAULT 0,
			rule_name TEXT NOT NULL DEFAULT '',
			entry_device_id INTEGER NOT NULL DEFAULT 0,
			entry_device_name TEXT NOT NULL DEFAULT '',
			entry_group_name TEXT NOT NULL DEFAULT '',
			protocol TEXT NOT NULL DEFAULT 'TCP',
			real_ip_source TEXT NOT NULL DEFAULT 'connection_log',
			connection_count INTEGER NOT NULL DEFAULT 1,
			first_seen TEXT NOT NULL,
			last_active_at TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			country TEXT NOT NULL DEFAULT '',
			user_agent TEXT NOT NULL DEFAULT '',
			remark TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS health_checks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			target_type TEXT NOT NULL DEFAULT 'device',
			target_id INTEGER NOT NULL DEFAULT 0,
			target_name TEXT NOT NULL DEFAULT '',
			protocol TEXT NOT NULL DEFAULT 'tcp',
			host TEXT NOT NULL,
			port INTEGER NOT NULL DEFAULT 0,
			path TEXT NOT NULL DEFAULT '',
			interval_sec INTEGER NOT NULL DEFAULT 60,
			timeout_sec INTEGER NOT NULL DEFAULT 5,
			status TEXT NOT NULL DEFAULT 'unknown',
			last_latency_ms INTEGER NOT NULL DEFAULT 0,
			last_error TEXT NOT NULL DEFAULT '',
			last_checked_at TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			remark TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS health_check_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			check_id INTEGER NOT NULL,
			status TEXT NOT NULL,
			latency_ms INTEGER NOT NULL DEFAULT 0,
			failure_reason TEXT NOT NULL DEFAULT '',
			checked_at TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS user_groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			sort INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			user_count INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS plans (
			id INTEGER PRIMARY KEY,
			sort INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			user_group TEXT NOT NULL,
			traffic TEXT NOT NULL,
			rule_limit INTEGER NOT NULL DEFAULT 0,
			price TEXT NOT NULL,
			hidden INTEGER NOT NULL DEFAULT 0,
			description TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS orders (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			created_at TEXT NOT NULL,
			paid_at TEXT NOT NULL DEFAULT '-',
			info TEXT NOT NULL,
			amount TEXT NOT NULL,
			type TEXT NOT NULL,
			status TEXT NOT NULL,
			commission TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS redeem_codes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT NOT NULL UNIQUE,
			plan TEXT NOT NULL,
			discount TEXT NOT NULL,
			remain INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS kv_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			actor TEXT NOT NULL DEFAULT 'system',
			action TEXT NOT NULL,
			resource TEXT NOT NULL,
			resource_id INTEGER NOT NULL DEFAULT 0,
			message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_status ON devices(status)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_group ON devices(group_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs(resource, resource_id)`,
		`CREATE INDEX IF NOT EXISTS idx_online_ips_rule ON online_ips(rule_id)`,
		`CREATE INDEX IF NOT EXISTS idx_online_ips_source ON online_ips(source_ip)`,
		`CREATE INDEX IF NOT EXISTS idx_online_ips_status ON online_ips(status)`,
		`CREATE INDEX IF NOT EXISTS idx_health_checks_status ON health_checks(status)`,
		`CREATE INDEX IF NOT EXISTS idx_health_results_check ON health_check_results(check_id, checked_at)`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	if err := migrateForwardRuleColumns(db); err != nil {
		return err
	}
	if err := migrateDeviceColumns(db); err != nil {
		return err
	}
	_, _ = db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, time.Now().UTC().Format(time.RFC3339))
	return nil
}

func migrateForwardRuleColumns(db *sql.DB) error {
	columns := map[string]string{
		"entry_group_id":      "INTEGER NOT NULL DEFAULT 0",
		"entry_group_name":    "TEXT NOT NULL DEFAULT ''",
		"listen_host":         "TEXT NOT NULL DEFAULT ''",
		"listen_port":         "TEXT NOT NULL DEFAULT ''",
		"exit_group_id":       "INTEGER NOT NULL DEFAULT 0",
		"exit_group_name":     "TEXT NOT NULL DEFAULT ''",
		"target_host":         "TEXT NOT NULL DEFAULT ''",
		"target_port":         "TEXT NOT NULL DEFAULT ''",
		"strategy":            "TEXT NOT NULL DEFAULT 'fallback'",
		"today_traffic":       "TEXT NOT NULL DEFAULT '0.00 GiB'",
		"sync_status":         "TEXT NOT NULL DEFAULT '未同步'",
		"proxy_protocol_mode": "TEXT NOT NULL DEFAULT 'send'",
		"current_connections": "INTEGER NOT NULL DEFAULT 0",
		"error_count":         "INTEGER NOT NULL DEFAULT 0",
		"last_hit_at":         "TEXT NOT NULL DEFAULT ''",
		"enabled":             "INTEGER NOT NULL DEFAULT 1",
		"remark":              "TEXT NOT NULL DEFAULT ''",
	}
	for name, definition := range columns {
		ok, err := columnExists(db, "forward_rules", name)
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		if _, err := db.Exec("ALTER TABLE forward_rules ADD COLUMN " + name + " " + definition); err != nil {
			return err
		}
	}
	_, err := db.Exec(`UPDATE forward_rules SET
		entry_group_name = CASE WHEN entry_group_name = '' THEN trim(substr(entry, 1, instr(entry || ':', ':') - 1)) ELSE entry_group_name END,
		listen_port = CASE WHEN listen_port = '' THEN trim(substr(entry, instr(entry || ':', ':') + 1)) ELSE listen_port END,
		exit_group_name = CASE WHEN exit_group_name = '' THEN trim(substr(exit, 1, instr(exit || ':', ':') - 1)) ELSE exit_group_name END,
		target_port = CASE WHEN target_port = '' THEN trim(substr(exit, instr(exit || ':', ':') + 1)) ELSE target_port END,
		sync_status = CASE
			WHEN status IN ('正常', '已暂停', '同步失败') THEN status
			WHEN sync_status = '' THEN status
			ELSE sync_status
		END,
		enabled = CASE WHEN status = '已暂停' THEN 0 ELSE enabled END`)
	return err
}

func migrateDeviceColumns(db *sql.DB) error {
	columns := map[string]string{
		"agent_token_hash":       "TEXT NOT NULL DEFAULT ''",
		"agent_registered_at":    "TEXT NOT NULL DEFAULT ''",
		"agent_token_expires_at": "TEXT NOT NULL DEFAULT ''",
		"agent_token_rotated_at": "TEXT NOT NULL DEFAULT ''",
		"config_version":         "INTEGER NOT NULL DEFAULT 1",
	}
	for name, definition := range columns {
		ok, err := columnExists(db, "devices", name)
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		if _, err := db.Exec("ALTER TABLE devices ADD COLUMN " + name + " " + definition); err != nil {
			return err
		}
	}
	_, err := db.Exec(`UPDATE devices SET config_version = 1 WHERE config_version = 0`)
	return err
}

func columnExists(db *sql.DB, table string, column string) (bool, error) {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func seed(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if empty, err := tableEmpty(tx, "users"); err != nil {
		return err
	} else if empty {
		adminPassword := strings.TrimSpace(os.Getenv("REPLEYPASS_ADMIN_PASSWORD"))
		if adminPassword == "" {
			return errors.New("REPLEYPASS_ADMIN_PASSWORD is required for first boot")
		}
		_, err = tx.Exec(`INSERT INTO users
			(username, password_hash, role, user_type, user_group, plan, plan_expire, traffic_used, traffic_total, max_rules, rate_limit, connection_limit, wallet_balance, telegram_linked, auto_renew_enabled, remark)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"admin", hashPassword(adminPassword), "super_admin", "管理员", "#1", "#55", "9999/9/9 上午8:09:09", "0.00 GiB", "55.00 GiB", 10, "500 Mbps", 300, "66 元", 0, 1, "")
		if err != nil {
			return err
		}
	}
	if empty, err := tableEmpty(tx, "forward_rules"); err != nil {
		return err
	} else if empty {
		rules := []forwardRule{
			{Name: "HK 低延迟入口", Entry: "香港入口 A : 20000", Exit: "东京出口组 : 443", EntryGroupID: 1, EntryGroupName: "香港入口 A", ListenHost: "", ListenPort: "20000", ExitGroupID: 2, ExitGroupName: "东京出口组", TargetPort: "443", Strategy: "fallback", UsedTraffic: "0.00 GiB", TodayTraffic: "0.00 GiB", Status: "正常", SyncStatus: "正常", Group: "未分组", Protocol: "TCP", Proxy: "v2 发送", ProxyProtocolMode: "send", Owner: "admin", Enabled: true},
			{Name: "WSS 回源", Entry: "新加坡入口 : 443", Exit: "美国 LA : 8443", EntryGroupName: "新加坡入口", ListenHost: "", ListenPort: "443", ExitGroupName: "美国 LA", TargetPort: "8443", Strategy: "ip_hash", UsedTraffic: "0.00 GiB", TodayTraffic: "0.00 GiB", Status: "未同步", SyncStatus: "未同步", Group: "Web", Protocol: "WSS", Proxy: "关闭", ProxyProtocolMode: "off", Owner: "admin", Enabled: true},
			{Name: "备用 TCP", Entry: "日本入口 : 18080", Exit: "德国出口组 : 18080", EntryGroupName: "日本入口", ListenHost: "", ListenPort: "18080", ExitGroupName: "德国出口组", TargetPort: "18080", Strategy: "least_conn", UsedTraffic: "0.00 GiB", TodayTraffic: "0.00 GiB", Status: "已暂停", SyncStatus: "已暂停", Group: "未分组", Protocol: "TCP", Proxy: "v1 发送", ProxyProtocolMode: "send", Owner: "admin", Enabled: false},
		}
		for _, rule := range rules {
			if _, err := tx.Exec(`INSERT INTO forward_rules (name, entry, exit, entry_group_id, entry_group_name, listen_host, listen_port, exit_group_id, exit_group_name, target_port, strategy, used_traffic, today_traffic, status, sync_status, rule_group, protocol, proxy_protocol, proxy_protocol_mode, owner, enabled) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				rule.Name, rule.Entry, rule.Exit, rule.EntryGroupID, rule.EntryGroupName, rule.ListenHost, rule.ListenPort, rule.ExitGroupID, rule.ExitGroupName, rule.TargetPort, rule.Strategy, rule.UsedTraffic, rule.TodayTraffic, rule.Status, rule.SyncStatus, rule.Group, rule.Protocol, rule.Proxy, rule.ProxyProtocolMode, rule.Owner, rule.Enabled); err != nil {
				return err
			}
		}
	}
	if empty, err := tableEmpty(tx, "device_groups"); err != nil {
		return err
	} else if empty {
		groups := []deviceGroup{
			{Sort: 1, Name: "香港入口 A", UserGroupID: "#1", Type: "入口", DisplayAddress: "hk.example.com", Multiplier: 1.5, UsedTraffic: "0.00 GiB", OnlineDevices: 0, Remark: "禁止接入"},
			{Sort: 2, Name: "东京出口组", UserGroupID: "#1", Type: "出口", DisplayAddress: "-", Multiplier: 0.5, UsedTraffic: "0.00 GiB", OnlineDevices: 0, Remark: "最小连接数"},
			{Sort: 3, Name: "私人单端 #1", UserGroupID: "#1", Type: "出口", DisplayAddress: "-", Multiplier: 1, UsedTraffic: "0.00 GiB", OnlineDevices: 0, Remark: "演示站禁止设备接入"},
		}
		for _, group := range groups {
			if _, err := tx.Exec(`INSERT INTO device_groups (sort, name, user_group_id, type, display_address, multiplier, used_traffic, online_devices, remark) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				group.Sort, group.Name, group.UserGroupID, group.Type, group.DisplayAddress, group.Multiplier, group.UsedTraffic, group.OnlineDevices, group.Remark); err != nil {
				return err
			}
		}
	}
	if empty, err := tableEmpty(tx, "devices"); err != nil {
		return err
	} else if empty {
		now := time.Now().UTC().Format(time.RFC3339)
		devices := []device{
			{Name: "hk-entry-01", GroupID: 1, GroupName: "香港入口 A", Type: "入口", Status: "offline", Address: "hk.example.com:20000", Region: "HK", Version: "edge-0.1.0", Load: "0%", LatencyMs: 0, ConnectionCount: 0, InboundTraffic: "0.00 GiB", OutboundTraffic: "0.00 GiB", LastHeartbeat: "", LastSeen: now, Enabled: true, Remark: "演示站禁止设备接入"},
			{Name: "tokyo-exit-01", GroupID: 2, GroupName: "东京出口组", Type: "出口", Status: "offline", Address: "tokyo-exit.example.com:443", Region: "JP", Version: "edge-0.1.0", Load: "0%", LatencyMs: 0, ConnectionCount: 0, InboundTraffic: "0.00 GiB", OutboundTraffic: "0.00 GiB", LastHeartbeat: "", LastSeen: now, Enabled: true, Remark: "等待真实节点心跳"},
			{Name: "private-single-01", GroupID: 3, GroupName: "私人单端 #1", Type: "出口", Status: "disabled", Address: "private-single.example.com:18080", Region: "Private", Version: "edge-0.1.0", Load: "0%", LatencyMs: 0, ConnectionCount: 0, InboundTraffic: "0.00 GiB", OutboundTraffic: "0.00 GiB", LastHeartbeat: "", LastSeen: now, Enabled: false, Remark: "用户单端隧道示例"},
		}
		for _, item := range devices {
			if _, err := tx.Exec(`INSERT INTO devices (name, group_id, group_name, type, status, address, region, version, load, latency_ms, connection_count, inbound_traffic, outbound_traffic, last_heartbeat, last_seen, enabled, remark) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				item.Name, item.GroupID, item.GroupName, item.Type, item.Status, item.Address, item.Region, item.Version, item.Load, item.LatencyMs, item.ConnectionCount, item.InboundTraffic, item.OutboundTraffic, item.LastHeartbeat, item.LastSeen, item.Enabled, item.Remark); err != nil {
				return err
			}
		}
	}
	if empty, err := tableEmpty(tx, "online_ips"); err != nil {
		return err
	} else if empty {
		now := time.Now().UTC().Format(time.RFC3339)
		items := []onlineIP{
			{SourceIP: "client-hk.example.com", SourcePort: 51234, RuleID: 1, RuleName: "HK 低延迟入口", EntryDeviceID: 1, EntryDeviceName: "hk-entry-01", EntryGroupName: "香港入口 A", Protocol: "TCP", RealIPSource: "proxy_protocol_v2", ConnectionCount: 2, FirstSeen: now, LastActiveAt: now, Status: "active", Country: "HK", Remark: "演示连接"},
			{SourceIP: "client-sg.example.com", SourcePort: 42311, RuleID: 2, RuleName: "WSS 回源", EntryDeviceID: 1, EntryDeviceName: "hk-entry-01", EntryGroupName: "香港入口 A", Protocol: "WSS", RealIPSource: "connection_log", ConnectionCount: 1, FirstSeen: now, LastActiveAt: now, Status: "active", Country: "SG", Remark: "演示连接"},
		}
		for _, item := range items {
			if _, err := tx.Exec(`INSERT INTO online_ips (source_ip, source_port, rule_id, rule_name, entry_device_id, entry_device_name, entry_group_name, protocol, real_ip_source, connection_count, first_seen, last_active_at, status, country, user_agent, remark) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				item.SourceIP, item.SourcePort, item.RuleID, item.RuleName, item.EntryDeviceID, item.EntryDeviceName, item.EntryGroupName, item.Protocol, item.RealIPSource, item.ConnectionCount, item.FirstSeen, item.LastActiveAt, item.Status, item.Country, item.UserAgent, item.Remark); err != nil {
				return err
			}
		}
	}
	if empty, err := tableEmpty(tx, "health_checks"); err != nil {
		return err
	} else if empty {
		now := time.Now().UTC().Format(time.RFC3339)
		checks := []healthCheck{
			{Name: "香港入口 TCP 探针", TargetType: "device", TargetID: 1, TargetName: "hk-entry-01", Protocol: "tcp", Host: "hk.example.com", Port: 20000, IntervalSec: 60, TimeoutSec: 5, Status: "healthy", LastLatencyMs: 18, LastCheckedAt: now, Enabled: true, Remark: "演示探针"},
			{Name: "东京出口 HTTPS 探针", TargetType: "device", TargetID: 2, TargetName: "tokyo-exit-01", Protocol: "https", Host: "tokyo-exit.example.com", Port: 443, Path: "/", IntervalSec: 60, TimeoutSec: 5, Status: "warning", LastLatencyMs: 96, LastError: "等待真实节点接入", LastCheckedAt: now, Enabled: true, Remark: "演示探针"},
			{Name: "备用规则同步探针", TargetType: "rule", TargetID: 3, TargetName: "备用 TCP", Protocol: "tcp", Host: "private-single.example.com", Port: 18080, IntervalSec: 120, TimeoutSec: 5, Status: "disabled", LastLatencyMs: 0, LastCheckedAt: now, Enabled: false, Remark: "规则已暂停"},
		}
		for _, item := range checks {
			result, err := tx.Exec(`INSERT INTO health_checks (name, target_type, target_id, target_name, protocol, host, port, path, interval_sec, timeout_sec, status, last_latency_ms, last_error, last_checked_at, enabled, remark) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				item.Name, item.TargetType, item.TargetID, item.TargetName, item.Protocol, item.Host, item.Port, item.Path, item.IntervalSec, item.TimeoutSec, item.Status, item.LastLatencyMs, item.LastError, item.LastCheckedAt, item.Enabled, item.Remark)
			if err != nil {
				return err
			}
			id, _ := result.LastInsertId()
			if _, err := tx.Exec(`INSERT INTO health_check_results (check_id, status, latency_ms, failure_reason, checked_at) VALUES (?, ?, ?, ?, ?)`,
				id, item.Status, item.LastLatencyMs, item.LastError, item.LastCheckedAt); err != nil {
				return err
			}
		}
	}
	if empty, err := tableEmpty(tx, "user_groups"); err != nil {
		return err
	} else if empty {
		if _, err := tx.Exec(`INSERT INTO user_groups (id, sort, name, user_count) VALUES (1, 1, '默认管理组', 1)`); err != nil {
			return err
		}
	}
	if empty, err := tableEmpty(tx, "plans"); err != nil {
		return err
	} else if empty {
		if _, err := tx.Exec(`INSERT INTO plans (id, sort, name, type, user_group, traffic, rule_limit, price, hidden, description) VALUES (55, 1, '演示套餐', '周期', '#1', '55.00 GiB', 10, '0 元', 0, '复刻参考站演示套餐')`); err != nil {
			return err
		}
	}
	if empty, err := tableEmpty(tx, "orders"); err != nil {
		return err
	} else if empty {
		if _, err := tx.Exec(`INSERT INTO orders (id, username, created_at, paid_at, info, amount, type, status) VALUES ('NP202607010001', 'admin', '2026/7/1 10:18:20', '-', '钱包充值', '66 元', '充值', '待支付')`); err != nil {
			return err
		}
	}
	if empty, err := tableEmpty(tx, "redeem_codes"); err != nil {
		return err
	} else if empty {
		if _, err := tx.Exec(`INSERT INTO redeem_codes (code, plan, discount, remain) VALUES ('DEMO-2026', '演示套餐', '100%', 10)`); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	_ = refreshRuntimeStats(db)
	return nil
}

func tableEmpty(tx *sql.Tx, table string) (bool, error) {
	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, response{
		Success: true,
		Data: map[string]interface{}{
			"status":        "ok",
			"uptimeSeconds": int(time.Since(s.startedAt).Seconds()),
			"store":         "sqlite",
		},
		RequestID: requestID(r),
	})
}

func (s *server) api(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeJSON(w, r, http.StatusOK, response{Success: true, RequestID: requestID(r)})
		return
	}

	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "/api/v1/auth/login" && r.Method == http.MethodPost {
		s.login(w, r)
		return
	}
	if path == "/api/v1/auth/logout" && r.Method == http.MethodPost {
		s.logout(w, r)
		return
	}
	if path == "/api/v1/auth/me" && r.Method == http.MethodGet {
		s.authMe(w, r)
		return
	}
	if strings.HasPrefix(path, "/api/v1/agent/") {
		s.agentRoute(w, r, path)
		return
	}

	if r.Method != http.MethodGet {
		if _, ok := s.currentUser(r); !ok {
			writeError(w, r, http.StatusUnauthorized, "login required")
			return
		}
	}

	if path == "/api/v1/rules/batch-import" {
		s.importRules(w, r)
		return
	}
	if path == "/api/v1/rules/batch-export" {
		s.exportRules(w, r)
		return
	}
	if strings.HasPrefix(path, "/api/v1/user/forward/") || strings.HasPrefix(path, "/api/v1/rules/") {
		s.ruleRoute(w, r, path)
		return
	}
	if strings.HasPrefix(path, "/api/v1/admin/devicegroup/") || strings.HasPrefix(path, "/api/v1/device-groups/") {
		s.deviceGroupResource(w, r, pathID(path))
		return
	}
	if strings.HasPrefix(path, "/api/v1/devices/") || strings.HasPrefix(path, "/api/v1/admin/device/") {
		s.deviceRoute(w, r, path)
		return
	}
	if strings.HasPrefix(path, "/api/v1/online-ips/") || strings.HasPrefix(path, "/api/v1/connections/") {
		s.onlineIPRoute(w, r, path)
		return
	}
	if strings.HasPrefix(path, "/api/v1/health-checks/") {
		s.healthCheckRoute(w, r, path)
		return
	}

	switch path {
	case "/api/v1/guest/kv/site_info":
		writeOK(w, r, map[string]interface{}{
			"siteName":          "demo",
			"allowRegister":     true,
			"invitePolicy":      "无限制",
			"captcha":           "无",
			"themePolicy":       "仅允许经典主题",
			"allowUserEndpoint": true,
			"allowLookingGlass": true,
		})
	case "/api/v1/system/info":
		writeOK(w, r, map[string]interface{}{
			"panel":            "nyanpass",
			"siteName":         "demo",
			"version":          "20260618",
			"licenseExpire":    "2286/11/21 上午1:46:39",
			"backend":          "RepleyPass Go API",
			"backendVersion":   "0.2.0",
			"deviceOnboarding": false,
			"store":            "SQLite",
		})
	case "/api/v1/system/info/queue":
		writeOK(w, r, map[string]interface{}{"tasks": 0, "scheduled": 0})
	case "/api/v1/system/node/status":
		s.nodeStatus(w, r)
	case "/api/v1/user/info":
		s.currentUserInfo(w, r)
	case "/api/v1/user/kv/site_notice":
		writeOK(w, r, map[string]string{"content": "这是演示站，已经禁止设备接入，您对接的设备不会上线。"})
	case "/api/v1/user/aff/config":
		writeOK(w, r, map[string]interface{}{"enabled": true, "rate": "0.00%", "balance": "33 元", "inviteURL": ""})
	case "/api/v1/user/devicegroup":
		s.userDeviceGroups(w, r)
	case "/api/v1/user/forward/folder":
		s.forwardFolders(w, r)
	case "/api/v1/user/forward", "/api/v1/rules":
		s.rules(w, r)
	case "/api/v1/user/shop/payment_info":
		writeOK(w, r, map[string]interface{}{"minRecharge": 0, "currency": "CNY", "channels": []interface{}{}})
	case "/api/v1/user/shop/plan", "/api/v1/admin/shop/plan":
		s.plans(w, r)
	case "/api/v1/user/shop/order", "/api/v1/admin/shop/order":
		s.orders(w, r)
	case "/api/v1/admin/statistic":
		s.statistic(w, r)
	case "/api/v1/admin/kv/site_notice":
		writeOK(w, r, map[string]string{"content": "这是演示站，已经禁止设备接入，您对接的设备不会上线。"})
	case "/api/v1/admin/kv/payment_info":
		writeOK(w, r, map[string]interface{}{"minRecharge": 0, "channels": []interface{}{}})
	case "/api/v1/admin/kv/invite_config":
		writeOK(w, r, map[string]interface{}{"enabled": false, "loopCommission": false, "forceTelegram": false, "rate": 0})
	case "/api/v1/admin/kv/telegram-bot-config":
		writeOK(w, r, map[string]interface{}{"enabled": false, "botToken": "", "webhookURL": "https://example.com/api/v1/telegram/webhook"})
	case "/api/v1/admin/kv/device-offline-notify-config":
		writeOK(w, r, map[string]interface{}{"globalGraceSeconds": 60, "globalKeepSeconds": 300, "overrides": []interface{}{}})
	case "/api/v1/admin/devicegroup", "/api/v1/device-groups":
		s.deviceGroups(w, r)
	case "/api/v1/admin/devicegroup/folder":
		s.deviceGroupFolders(w, r)
	case "/api/v1/devices", "/api/v1/admin/device":
		s.devices(w, r)
	case "/api/v1/online-ips", "/api/v1/connections":
		s.onlineIPs(w, r)
	case "/api/v1/health-checks":
		s.healthChecks(w, r)
	case "/api/v1/admin/user", "/api/v1/users":
		s.users(w, r)
	case "/api/v1/admin/usergroup":
		s.userGroups(w, r)
	case "/api/v1/admin/shop/redeem":
		s.redeemCodes(w, r)
	case "/api/v1/admin/aff/log":
		logs := []order{{ID: "AFF202607010001", User: "admin", CreatedAt: "2026/7/1 10:18:20", Info: "手动记账", Amount: "33 元", Type: "返佣", Status: "已记录"}}
		writeOK(w, r, paginate(r, logs))
	case "/api/v1/logs/audit":
		s.auditLogs(w, r)
	case "/api/v1/dashboard/overview":
		s.dashboardOverview(w, r)
	case "/api/v1/dashboard/topology":
		s.topology(w, r)
	default:
		writeError(w, r, http.StatusNotFound, "endpoint not found")
	}
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid json")
		return
	}
	var storedHash string
	var u user
	row := s.db.QueryRow(`SELECT id, id, username, role, user_type, user_group, plan, plan_expire, traffic_used, traffic_total, max_rules, rate_limit, connection_limit, wallet_balance, telegram_linked, auto_renew_enabled, remark, password_hash FROM users WHERE username = ?`, req.Username)
	if err := row.Scan(&u.ID, &u.UID, &u.Username, &u.Role, &u.UserType, &u.UserGroup, &u.Plan, &u.PlanExpire, &u.TrafficUsed, &u.TrafficTotal, &u.MaxRules, &u.RateLimit, &u.ConnectionLimit, &u.WalletBalance, &u.TelegramLinked, &u.AutoRenewEnabled, &u.Remark, &storedHash); err != nil {
		writeError(w, r, http.StatusUnauthorized, "invalid username or password")
		return
	}
	if storedHash != hashPassword(req.Password) {
		writeError(w, r, http.StatusUnauthorized, "invalid username or password")
		return
	}
	token, err := randomToken()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "create session failed")
		return
	}
	expiresAt := time.Now().Add(24 * time.Hour).UTC()
	if _, err := s.db.Exec(`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`, token, u.ID, expiresAt.Format(time.RFC3339)); err != nil {
		writeError(w, r, http.StatusInternalServerError, "create session failed")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int((24 * time.Hour).Seconds()),
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeOK(w, r, map[string]interface{}{"user": u, "token": token, "expiresAt": expiresAt.Format(time.RFC3339)})
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if token := sessionToken(r); token != "" {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	writeOK(w, r, map[string]bool{"loggedOut": true})
}

func (s *server) authMe(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "login required")
		return
	}
	writeOK(w, r, u)
}

func (s *server) currentUserInfo(w http.ResponseWriter, r *http.Request) {
	if u, ok := s.currentUser(r); ok {
		writeOK(w, r, u)
		return
	}
	u, err := s.userByUsername("admin")
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "user not found")
		return
	}
	writeOK(w, r, u)
}

func (s *server) currentUser(r *http.Request) (user, bool) {
	token := sessionToken(r)
	if token == "" {
		return user{}, false
	}
	var userID int
	var expiresAt string
	err := s.db.QueryRow(`SELECT user_id, expires_at FROM sessions WHERE token = ?`, token).Scan(&userID, &expiresAt)
	if err != nil {
		return user{}, false
	}
	expires, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || time.Now().UTC().After(expires) {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
		return user{}, false
	}
	u, err := s.userByID(userID)
	return u, err == nil
}

func sessionToken(r *http.Request) string {
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	return ""
}

func (s *server) userByUsername(username string) (user, error) {
	var u user
	err := s.db.QueryRow(`SELECT id, id, username, role, user_type, user_group, plan, plan_expire, traffic_used, traffic_total, max_rules, rate_limit, connection_limit, wallet_balance, telegram_linked, auto_renew_enabled, remark FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.UID, &u.Username, &u.Role, &u.UserType, &u.UserGroup, &u.Plan, &u.PlanExpire, &u.TrafficUsed, &u.TrafficTotal, &u.MaxRules, &u.RateLimit, &u.ConnectionLimit, &u.WalletBalance, &u.TelegramLinked, &u.AutoRenewEnabled, &u.Remark)
	return u, err
}

func (s *server) userByID(id int) (user, error) {
	var u user
	err := s.db.QueryRow(`SELECT id, id, username, role, user_type, user_group, plan, plan_expire, traffic_used, traffic_total, max_rules, rate_limit, connection_limit, wallet_balance, telegram_linked, auto_renew_enabled, remark FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.UID, &u.Username, &u.Role, &u.UserType, &u.UserGroup, &u.Plan, &u.PlanExpire, &u.TrafficUsed, &u.TrafficTotal, &u.MaxRules, &u.RateLimit, &u.ConnectionLimit, &u.WalletBalance, &u.TelegramLinked, &u.AutoRenewEnabled, &u.Remark)
	return u, err
}

func (s *server) rules(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.createRule(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	rules, err := s.listRules()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "list rules failed")
		return
	}
	writeOK(w, r, paginate(r, rules))
}

func (s *server) ruleRoute(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 {
		writeError(w, r, http.StatusBadRequest, "invalid rule route")
		return
	}
	id, err := strconv.Atoi(parts[len(parts)-1])
	action := ""
	if err != nil && len(parts) >= 5 {
		action = parts[len(parts)-1]
		id, err = strconv.Atoi(parts[len(parts)-2])
	}
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusBadRequest, "invalid rule id")
		return
	}
	if action != "" {
		s.ruleAction(w, r, id, action)
		return
	}
	s.ruleResource(w, r, id)
}

func (s *server) ruleResource(w http.ResponseWriter, r *http.Request, id int) {
	if id <= 0 {
		writeError(w, r, http.StatusBadRequest, "invalid rule id")
		return
	}
	switch r.Method {
	case http.MethodGet:
		rule, err := s.ruleByID(id)
		if err != nil {
			writeError(w, r, http.StatusNotFound, "rule not found")
			return
		}
		writeOK(w, r, rule)
	case http.MethodPatch, http.MethodPut:
		s.updateRule(w, r, id)
	case http.MethodDelete:
		if _, err := s.db.Exec(`DELETE FROM forward_rules WHERE id = ?`, id); err != nil {
			writeError(w, r, http.StatusInternalServerError, "delete rule failed")
			return
		}
		_ = s.bumpAgentConfigVersions()
		s.logAudit(r, "delete", "rule", id, "删除转发规则")
		writeOK(w, r, map[string]interface{}{"deleted": true, "id": id})
	default:
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) ruleAction(w http.ResponseWriter, r *http.Request, id int, action string) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var status string
	var syncStatus string
	var enabled bool
	var message string
	switch action {
	case "start", "enable":
		status, syncStatus, enabled, message = "未同步", "未同步", true, "启动转发规则"
	case "pause", "disable":
		status, syncStatus, enabled, message = "已暂停", "已暂停", false, "暂停转发规则"
	case "sync":
		status, syncStatus, enabled, message = "正常", "正常", true, "同步转发规则"
	default:
		writeError(w, r, http.StatusNotFound, "rule action not found")
		return
	}
	if _, err := s.db.Exec(`UPDATE forward_rules SET status = ?, sync_status = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, syncStatus, enabled, id); err != nil {
		writeError(w, r, http.StatusInternalServerError, "update rule status failed")
		return
	}
	_ = s.bumpAgentConfigVersions()
	s.logAudit(r, action, "rule", id, message)
	rule, err := s.ruleByID(id)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "rule not found")
		return
	}
	writeOK(w, r, rule)
}

func (s *server) createRule(w http.ResponseWriter, r *http.Request) {
	rule, err := decodeRule(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.db.Exec(`INSERT INTO forward_rules (name, entry, exit, entry_group_id, entry_group_name, listen_host, listen_port, exit_group_id, exit_group_name, target_host, target_port, strategy, used_traffic, today_traffic, status, sync_status, rule_group, protocol, proxy_protocol, proxy_protocol_mode, owner, current_connections, error_count, last_hit_at, enabled, remark) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.Name, rule.Entry, rule.Exit, rule.EntryGroupID, rule.EntryGroupName, rule.ListenHost, rule.ListenPort, rule.ExitGroupID, rule.ExitGroupName, rule.TargetHost, rule.TargetPort, rule.Strategy, rule.UsedTraffic, rule.TodayTraffic, rule.Status, rule.SyncStatus, rule.Group, rule.Protocol, rule.Proxy, rule.ProxyProtocolMode, rule.Owner, rule.CurrentConnections, rule.ErrorCount, rule.LastHitAt, rule.Enabled, rule.Remark)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "create rule failed")
		return
	}
	id, _ := result.LastInsertId()
	_ = s.bumpAgentConfigVersions()
	created, _ := s.ruleByID(int(id))
	s.logAudit(r, "create", "rule", int(id), "创建转发规则")
	writeJSON(w, r, http.StatusCreated, response{Success: true, Data: created, RequestID: requestID(r)})
}

func (s *server) updateRule(w http.ResponseWriter, r *http.Request, id int) {
	current, err := s.ruleByID(id)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "rule not found")
		return
	}
	patch, err := decodeRule(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if patch.Name != "" {
		current.Name = patch.Name
	}
	if patch.Entry != "" {
		current.Entry = patch.Entry
	}
	if patch.Exit != "" {
		current.Exit = patch.Exit
	}
	if patch.EntryGroupID != 0 {
		current.EntryGroupID = patch.EntryGroupID
	}
	if patch.EntryGroupName != "" {
		current.EntryGroupName = patch.EntryGroupName
	}
	if patch.ListenHost != "" {
		current.ListenHost = patch.ListenHost
	}
	if patch.ListenPort != "" {
		current.ListenPort = patch.ListenPort
	}
	if patch.ExitGroupID != 0 {
		current.ExitGroupID = patch.ExitGroupID
	}
	if patch.ExitGroupName != "" {
		current.ExitGroupName = patch.ExitGroupName
	}
	if patch.TargetHost != "" {
		current.TargetHost = patch.TargetHost
	}
	if patch.TargetPort != "" {
		current.TargetPort = patch.TargetPort
	}
	if patch.Strategy != "" {
		current.Strategy = patch.Strategy
	}
	if patch.UsedTraffic != "" {
		current.UsedTraffic = patch.UsedTraffic
	}
	if patch.TodayTraffic != "" {
		current.TodayTraffic = patch.TodayTraffic
	}
	if patch.Status != "" {
		current.Status = patch.Status
	}
	if patch.SyncStatus != "" {
		current.SyncStatus = patch.SyncStatus
	}
	if patch.Group != "" {
		current.Group = patch.Group
	}
	if patch.Protocol != "" {
		current.Protocol = patch.Protocol
	}
	if patch.Proxy != "" {
		current.Proxy = patch.Proxy
	}
	if patch.ProxyProtocolMode != "" {
		current.ProxyProtocolMode = patch.ProxyProtocolMode
	}
	if patch.Owner != "" {
		current.Owner = patch.Owner
	}
	if patch.CurrentConnections != 0 {
		current.CurrentConnections = patch.CurrentConnections
	}
	if patch.ErrorCount != 0 {
		current.ErrorCount = patch.ErrorCount
	}
	if patch.LastHitAt != "" {
		current.LastHitAt = patch.LastHitAt
	}
	if patch.Remark != "" {
		current.Remark = patch.Remark
	}
	current.Entry = composeEndpoint(current.EntryGroupName, current.ListenPort)
	current.Exit = composeEndpoint(firstNonEmpty(current.ExitGroupName, current.TargetHost), current.TargetPort)
	current.Enabled = current.Status != "已暂停"
	if _, err := s.db.Exec(`UPDATE forward_rules SET name = ?, entry = ?, exit = ?, entry_group_id = ?, entry_group_name = ?, listen_host = ?, listen_port = ?, exit_group_id = ?, exit_group_name = ?, target_host = ?, target_port = ?, strategy = ?, used_traffic = ?, today_traffic = ?, status = ?, sync_status = ?, rule_group = ?, protocol = ?, proxy_protocol = ?, proxy_protocol_mode = ?, owner = ?, current_connections = ?, error_count = ?, last_hit_at = ?, enabled = ?, remark = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		current.Name, current.Entry, current.Exit, current.EntryGroupID, current.EntryGroupName, current.ListenHost, current.ListenPort, current.ExitGroupID, current.ExitGroupName, current.TargetHost, current.TargetPort, current.Strategy, current.UsedTraffic, current.TodayTraffic, current.Status, current.SyncStatus, current.Group, current.Protocol, current.Proxy, current.ProxyProtocolMode, current.Owner, current.CurrentConnections, current.ErrorCount, current.LastHitAt, current.Enabled, current.Remark, id); err != nil {
		writeError(w, r, http.StatusInternalServerError, "update rule failed")
		return
	}
	_ = s.bumpAgentConfigVersions()
	s.logAudit(r, "update", "rule", id, "修改转发规则")
	writeOK(w, r, current)
}

func decodeRule(r *http.Request) (forwardRule, error) {
	var rule forwardRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		return rule, errors.New("invalid json")
	}
	rule.Name = strings.TrimSpace(rule.Name)
	rule.Entry = strings.TrimSpace(rule.Entry)
	rule.Exit = strings.TrimSpace(rule.Exit)
	rule.EntryGroupName = strings.TrimSpace(rule.EntryGroupName)
	rule.ListenHost = strings.TrimSpace(rule.ListenHost)
	rule.ListenPort = strings.TrimSpace(rule.ListenPort)
	rule.ExitGroupName = strings.TrimSpace(rule.ExitGroupName)
	rule.TargetHost = strings.TrimSpace(rule.TargetHost)
	rule.TargetPort = strings.TrimSpace(rule.TargetPort)
	rule.Strategy = strings.TrimSpace(rule.Strategy)
	rule.SyncStatus = strings.TrimSpace(rule.SyncStatus)
	rule.ProxyProtocolMode = strings.TrimSpace(rule.ProxyProtocolMode)
	rule.Remark = strings.TrimSpace(rule.Remark)
	if r.Method == http.MethodPost && rule.Name == "" {
		return rule, errors.New("name is required")
	}
	if r.Method == http.MethodPost {
		if rule.EntryGroupName == "" && rule.Entry != "" {
			rule.EntryGroupName, rule.ListenPort = splitEndpoint(rule.Entry)
		}
		if rule.ExitGroupName == "" && rule.Exit != "" {
			rule.ExitGroupName, rule.TargetPort = splitEndpoint(rule.Exit)
		}
		if rule.EntryGroupName == "" || rule.ListenPort == "" || rule.ExitGroupName == "" || rule.TargetPort == "" {
			return rule, errors.New("entry group, listen port, exit group and target port are required")
		}
		if rule.ListenHost == "" {
			rule.ListenHost = ""
		}
		if rule.Strategy == "" {
			rule.Strategy = "fallback"
		}
		if rule.Entry == "" {
			rule.Entry = composeEndpoint(rule.EntryGroupName, rule.ListenPort)
		}
		if rule.Exit == "" {
			rule.Exit = composeEndpoint(firstNonEmpty(rule.ExitGroupName, rule.TargetHost), rule.TargetPort)
		}
		if rule.UsedTraffic == "" {
			rule.UsedTraffic = "0.00 GiB"
		}
		if rule.TodayTraffic == "" {
			rule.TodayTraffic = "0.00 GiB"
		}
		if rule.Status == "" {
			rule.Status = "未同步"
		}
		if rule.SyncStatus == "" {
			rule.SyncStatus = rule.Status
		}
		if rule.Group == "" {
			rule.Group = "未分组"
		}
		if rule.Protocol == "" {
			rule.Protocol = "TCP"
		}
		if rule.Proxy == "" {
			rule.Proxy = "关闭"
		}
		if rule.ProxyProtocolMode == "" {
			rule.ProxyProtocolMode = "send"
		}
		if rule.Owner == "" {
			rule.Owner = "admin"
		}
		rule.Enabled = rule.Status != "已暂停"
	}
	return rule, nil
}

func (s *server) listRules() ([]forwardRule, error) {
	rows, err := s.db.Query(ruleSelectSQL + ` ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []forwardRule
	for rows.Next() {
		var item forwardRule
		if err := scanRule(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *server) ruleByID(id int) (forwardRule, error) {
	var item forwardRule
	err := scanRule(s.db.QueryRow(ruleSelectSQL+` WHERE id = ?`, id), &item)
	return item, err
}

const ruleSelectSQL = `SELECT id, name, entry, exit, entry_group_id, entry_group_name, listen_host, listen_port, exit_group_id, exit_group_name, target_host, target_port, strategy, used_traffic, today_traffic, status, sync_status, rule_group, protocol, proxy_protocol, proxy_protocol_mode, owner, current_connections, error_count, last_hit_at, enabled, remark FROM forward_rules`

type sqlScanner interface {
	Scan(dest ...interface{}) error
}

func scanRule(scanner sqlScanner, item *forwardRule) error {
	return scanner.Scan(&item.ID, &item.Name, &item.Entry, &item.Exit, &item.EntryGroupID, &item.EntryGroupName, &item.ListenHost, &item.ListenPort, &item.ExitGroupID, &item.ExitGroupName, &item.TargetHost, &item.TargetPort, &item.Strategy, &item.UsedTraffic, &item.TodayTraffic, &item.Status, &item.SyncStatus, &item.Group, &item.Protocol, &item.Proxy, &item.ProxyProtocolMode, &item.Owner, &item.CurrentConnections, &item.ErrorCount, &item.LastHitAt, &item.Enabled, &item.Remark)
}

func (s *server) importRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		Rules []forwardRule `json:"rules"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid json")
		return
	}
	if len(payload.Rules) == 0 {
		writeError(w, r, http.StatusBadRequest, "rules are required")
		return
	}
	created := []forwardRule{}
	for _, input := range payload.Rules {
		input.normalizeForCreate()
		if input.Name == "" || input.EntryGroupName == "" || input.ListenPort == "" || input.ExitGroupName == "" || input.TargetPort == "" {
			writeError(w, r, http.StatusBadRequest, "each rule requires name, entry group, listen port, exit group and target port")
			return
		}
		result, err := s.db.Exec(`INSERT INTO forward_rules (name, entry, exit, entry_group_id, entry_group_name, listen_host, listen_port, exit_group_id, exit_group_name, target_host, target_port, strategy, used_traffic, today_traffic, status, sync_status, rule_group, protocol, proxy_protocol, proxy_protocol_mode, owner, current_connections, error_count, last_hit_at, enabled, remark) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			input.Name, input.Entry, input.Exit, input.EntryGroupID, input.EntryGroupName, input.ListenHost, input.ListenPort, input.ExitGroupID, input.ExitGroupName, input.TargetHost, input.TargetPort, input.Strategy, input.UsedTraffic, input.TodayTraffic, input.Status, input.SyncStatus, input.Group, input.Protocol, input.Proxy, input.ProxyProtocolMode, input.Owner, input.CurrentConnections, input.ErrorCount, input.LastHitAt, input.Enabled, input.Remark)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "import rule failed")
			return
		}
		id, _ := result.LastInsertId()
		rule, _ := s.ruleByID(int(id))
		created = append(created, rule)
		s.logAudit(r, "import", "rule", int(id), "批量导入转发规则")
	}
	_ = s.bumpAgentConfigVersions()
	writeJSON(w, r, http.StatusCreated, response{Success: true, Data: map[string]interface{}{"items": created, "total": len(created)}, RequestID: requestID(r)})
}

func (s *server) exportRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	rules, err := s.listRules()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "export rules failed")
		return
	}
	writeOK(w, r, map[string]interface{}{"rules": rules, "exportedAt": time.Now().UTC().Format(time.RFC3339)})
}

func (rule *forwardRule) normalizeForCreate() {
	rule.Name = strings.TrimSpace(rule.Name)
	rule.Entry = strings.TrimSpace(rule.Entry)
	rule.Exit = strings.TrimSpace(rule.Exit)
	rule.EntryGroupName = strings.TrimSpace(rule.EntryGroupName)
	rule.ListenHost = strings.TrimSpace(rule.ListenHost)
	rule.ListenPort = strings.TrimSpace(rule.ListenPort)
	rule.ExitGroupName = strings.TrimSpace(rule.ExitGroupName)
	rule.TargetHost = strings.TrimSpace(rule.TargetHost)
	rule.TargetPort = strings.TrimSpace(rule.TargetPort)
	if rule.EntryGroupName == "" && rule.Entry != "" {
		rule.EntryGroupName, rule.ListenPort = splitEndpoint(rule.Entry)
	}
	if rule.ExitGroupName == "" && rule.Exit != "" {
		rule.ExitGroupName, rule.TargetPort = splitEndpoint(rule.Exit)
	}
	if rule.ListenHost == "" {
		rule.ListenHost = ""
	}
	if rule.Strategy == "" {
		rule.Strategy = "fallback"
	}
	if rule.Entry == "" {
		rule.Entry = composeEndpoint(rule.EntryGroupName, rule.ListenPort)
	}
	if rule.Exit == "" {
		rule.Exit = composeEndpoint(firstNonEmpty(rule.ExitGroupName, rule.TargetHost), rule.TargetPort)
	}
	if rule.UsedTraffic == "" {
		rule.UsedTraffic = "0.00 GiB"
	}
	if rule.TodayTraffic == "" {
		rule.TodayTraffic = "0.00 GiB"
	}
	if rule.Status == "" {
		rule.Status = "未同步"
	}
	if rule.SyncStatus == "" {
		rule.SyncStatus = rule.Status
	}
	if rule.Group == "" {
		rule.Group = "未分组"
	}
	if rule.Protocol == "" {
		rule.Protocol = "TCP"
	}
	if rule.Proxy == "" {
		rule.Proxy = "关闭"
	}
	if rule.ProxyProtocolMode == "" {
		rule.ProxyProtocolMode = "send"
	}
	if rule.Owner == "" {
		rule.Owner = "admin"
	}
	rule.Enabled = rule.Status != "已暂停"
}

func (s *server) deviceGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.createDeviceGroup(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	groups, err := s.listDeviceGroups(false)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "list device groups failed")
		return
	}
	writeOK(w, r, groups)
}

func (s *server) userDeviceGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := s.listDeviceGroups(true)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "list device groups failed")
		return
	}
	writeOK(w, r, groups)
}

func (s *server) deviceGroupResource(w http.ResponseWriter, r *http.Request, id int) {
	if id <= 0 {
		writeError(w, r, http.StatusBadRequest, "invalid device group id")
		return
	}
	switch r.Method {
	case http.MethodGet:
		group, err := s.deviceGroupByID(id)
		if err != nil {
			writeError(w, r, http.StatusNotFound, "device group not found")
			return
		}
		writeOK(w, r, group)
	case http.MethodPatch, http.MethodPut:
		s.updateDeviceGroup(w, r, id)
	case http.MethodDelete:
		if _, err := s.db.Exec(`DELETE FROM device_groups WHERE id = ?`, id); err != nil {
			writeError(w, r, http.StatusInternalServerError, "delete device group failed")
			return
		}
		_ = s.bumpAgentConfigVersions()
		writeOK(w, r, map[string]interface{}{"deleted": true, "id": id})
	default:
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) createDeviceGroup(w http.ResponseWriter, r *http.Request) {
	group, err := decodeDeviceGroup(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.db.Exec(`INSERT INTO device_groups (sort, name, user_group_id, type, display_address, multiplier, used_traffic, online_devices, remark) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		group.Sort, group.Name, group.UserGroupID, group.Type, group.DisplayAddress, group.Multiplier, group.UsedTraffic, group.OnlineDevices, group.Remark)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "create device group failed")
		return
	}
	id, _ := result.LastInsertId()
	_ = s.bumpAgentConfigVersions()
	created, _ := s.deviceGroupByID(int(id))
	writeJSON(w, r, http.StatusCreated, response{Success: true, Data: created, RequestID: requestID(r)})
}

func (s *server) updateDeviceGroup(w http.ResponseWriter, r *http.Request, id int) {
	current, err := s.deviceGroupByID(id)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "device group not found")
		return
	}
	patch, err := decodeDeviceGroup(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if patch.Sort != 0 {
		current.Sort = patch.Sort
	}
	if patch.Name != "" {
		current.Name = patch.Name
	}
	if patch.UserGroupID != "" {
		current.UserGroupID = patch.UserGroupID
	}
	if patch.Type != "" {
		current.Type = patch.Type
	}
	if patch.DisplayAddress != "" {
		current.DisplayAddress = patch.DisplayAddress
	}
	if patch.Multiplier != 0 {
		current.Multiplier = patch.Multiplier
	}
	if patch.UsedTraffic != "" {
		current.UsedTraffic = patch.UsedTraffic
	}
	if patch.OnlineDevices != 0 {
		current.OnlineDevices = patch.OnlineDevices
	}
	if patch.Remark != "" {
		current.Remark = patch.Remark
	}
	if _, err := s.db.Exec(`UPDATE device_groups SET sort = ?, name = ?, user_group_id = ?, type = ?, display_address = ?, multiplier = ?, used_traffic = ?, online_devices = ?, remark = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		current.Sort, current.Name, current.UserGroupID, current.Type, current.DisplayAddress, current.Multiplier, current.UsedTraffic, current.OnlineDevices, current.Remark, id); err != nil {
		writeError(w, r, http.StatusInternalServerError, "update device group failed")
		return
	}
	_ = s.bumpAgentConfigVersions()
	writeOK(w, r, current)
}

func decodeDeviceGroup(r *http.Request) (deviceGroup, error) {
	var group deviceGroup
	if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
		return group, errors.New("invalid json")
	}
	group.Name = strings.TrimSpace(group.Name)
	if r.Method == http.MethodPost && group.Name == "" {
		return group, errors.New("name is required")
	}
	if r.Method == http.MethodPost {
		if group.UserGroupID == "" {
			group.UserGroupID = "#1"
		}
		if group.Type == "" {
			group.Type = "出口"
		}
		if group.DisplayAddress == "" {
			group.DisplayAddress = "-"
		}
		if group.Multiplier == 0 {
			group.Multiplier = 1
		}
		if group.UsedTraffic == "" {
			group.UsedTraffic = "0.00 GiB"
		}
	}
	return group, nil
}

func (s *server) listDeviceGroups(userOnly bool) ([]deviceGroup, error) {
	query := `SELECT id, sort, name, user_group_id, type, display_address, multiplier, used_traffic, online_devices, remark FROM device_groups`
	if userOnly {
		query += ` WHERE name LIKE '%私人%' OR remark LIKE '%单端%'`
	}
	query += ` ORDER BY sort ASC, id ASC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []deviceGroup
	for rows.Next() {
		var item deviceGroup
		if err := rows.Scan(&item.ID, &item.Sort, &item.Name, &item.UserGroupID, &item.Type, &item.DisplayAddress, &item.Multiplier, &item.UsedTraffic, &item.OnlineDevices, &item.Remark); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *server) deviceGroupByID(id int) (deviceGroup, error) {
	var item deviceGroup
	err := s.db.QueryRow(`SELECT id, sort, name, user_group_id, type, display_address, multiplier, used_traffic, online_devices, remark FROM device_groups WHERE id = ?`, id).
		Scan(&item.ID, &item.Sort, &item.Name, &item.UserGroupID, &item.Type, &item.DisplayAddress, &item.Multiplier, &item.UsedTraffic, &item.OnlineDevices, &item.Remark)
	return item, err
}

func (s *server) devices(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.createDevice(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	items, err := s.listDevices(r)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "list devices failed")
		return
	}
	writeOK(w, r, paginate(r, items))
}

func (s *server) deviceRoute(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 {
		writeError(w, r, http.StatusBadRequest, "invalid device route")
		return
	}
	id, err := strconv.Atoi(parts[len(parts)-1])
	action := ""
	if err != nil && len(parts) >= 5 {
		action = parts[len(parts)-1]
		id, err = strconv.Atoi(parts[len(parts)-2])
	}
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusBadRequest, "invalid device id")
		return
	}
	if action != "" {
		s.deviceAction(w, r, id, action)
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := s.deviceByID(id)
		if err != nil {
			writeError(w, r, http.StatusNotFound, "device not found")
			return
		}
		writeOK(w, r, item)
	case http.MethodPatch, http.MethodPut:
		s.updateDevice(w, r, id)
	case http.MethodDelete:
		if _, err := s.db.Exec(`DELETE FROM devices WHERE id = ?`, id); err != nil {
			writeError(w, r, http.StatusInternalServerError, "delete device failed")
			return
		}
		_ = s.recountDeviceGroupOnline()
		writeOK(w, r, map[string]interface{}{"deleted": true, "id": id})
	default:
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) deviceAction(w http.ResponseWriter, r *http.Request, id int, action string) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	switch action {
	case "enable":
		if _, err := s.db.Exec(`UPDATE devices SET enabled = 1, status = CASE WHEN status = 'disabled' THEN 'offline' ELSE status END, config_version = config_version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
			writeError(w, r, http.StatusInternalServerError, "enable device failed")
			return
		}
	case "disable":
		if _, err := s.db.Exec(`UPDATE devices SET enabled = 0, status = 'disabled', connection_count = 0, config_version = config_version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
			writeError(w, r, http.StatusInternalServerError, "disable device failed")
			return
		}
	case "heartbeat":
		if err := s.heartbeatDevice(r, id); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error())
			return
		}
	case "agent-token":
		s.issueAgentToken(w, r, id)
		return
	default:
		writeError(w, r, http.StatusNotFound, "device action not found")
		return
	}
	_ = s.recountDeviceGroupOnline()
	item, err := s.deviceByID(id)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "device not found")
		return
	}
	writeOK(w, r, item)
}

func (s *server) createDevice(w http.ResponseWriter, r *http.Request) {
	item, err := decodeDevice(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.db.Exec(`INSERT INTO devices (name, group_id, group_name, type, status, address, region, version, load, latency_ms, connection_count, inbound_traffic, outbound_traffic, last_heartbeat, last_seen, enabled, remark) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.Name, item.GroupID, item.GroupName, item.Type, item.Status, item.Address, item.Region, item.Version, item.Load, item.LatencyMs, item.ConnectionCount, item.InboundTraffic, item.OutboundTraffic, item.LastHeartbeat, item.LastSeen, item.Enabled, item.Remark)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "create device failed")
		return
	}
	id, _ := result.LastInsertId()
	_ = s.recountDeviceGroupOnline()
	created, _ := s.deviceByID(int(id))
	writeJSON(w, r, http.StatusCreated, response{Success: true, Data: created, RequestID: requestID(r)})
}

func (s *server) updateDevice(w http.ResponseWriter, r *http.Request, id int) {
	current, err := s.deviceByID(id)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "device not found")
		return
	}
	patch, err := decodeDevice(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if patch.Name != "" {
		current.Name = patch.Name
	}
	if patch.GroupID != 0 {
		current.GroupID = patch.GroupID
	}
	if patch.GroupName != "" {
		current.GroupName = patch.GroupName
	}
	if patch.Type != "" {
		current.Type = patch.Type
	}
	if patch.Status != "" {
		current.Status = patch.Status
	}
	if patch.Address != "" {
		current.Address = patch.Address
	}
	if patch.Region != "" {
		current.Region = patch.Region
	}
	if patch.Version != "" {
		current.Version = patch.Version
	}
	if patch.Load != "" {
		current.Load = patch.Load
	}
	if patch.LatencyMs != 0 {
		current.LatencyMs = patch.LatencyMs
	}
	if patch.ConnectionCount != 0 {
		current.ConnectionCount = patch.ConnectionCount
	}
	if patch.InboundTraffic != "" {
		current.InboundTraffic = patch.InboundTraffic
	}
	if patch.OutboundTraffic != "" {
		current.OutboundTraffic = patch.OutboundTraffic
	}
	if patch.LastHeartbeat != "" {
		current.LastHeartbeat = patch.LastHeartbeat
	}
	if patch.LastSeen != "" {
		current.LastSeen = patch.LastSeen
	}
	if patch.Remark != "" {
		current.Remark = patch.Remark
	}
	current.Enabled = current.Status != "disabled"
	if _, err := s.db.Exec(`UPDATE devices SET name = ?, group_id = ?, group_name = ?, type = ?, status = ?, address = ?, region = ?, version = ?, load = ?, latency_ms = ?, connection_count = ?, inbound_traffic = ?, outbound_traffic = ?, last_heartbeat = ?, last_seen = ?, enabled = ?, config_version = config_version + 1, remark = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		current.Name, current.GroupID, current.GroupName, current.Type, current.Status, current.Address, current.Region, current.Version, current.Load, current.LatencyMs, current.ConnectionCount, current.InboundTraffic, current.OutboundTraffic, current.LastHeartbeat, current.LastSeen, current.Enabled, current.Remark, id); err != nil {
		writeError(w, r, http.StatusInternalServerError, "update device failed")
		return
	}
	_ = s.recountDeviceGroupOnline()
	current.ConfigVersion++
	writeOK(w, r, current)
}

func (s *server) issueAgentToken(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, err := s.deviceByID(id); err != nil {
		writeError(w, r, http.StatusNotFound, "device not found")
		return
	}
	expiresAt, err := decodeAgentTokenExpiry(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	token, err := randomToken()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "create agent token failed")
		return
	}
	rotatedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`UPDATE devices SET agent_token_hash = ?, agent_token_expires_at = ?, agent_token_rotated_at = ?, config_version = config_version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, hashAgentToken(token), expiresAt.Format(time.RFC3339), rotatedAt, id); err != nil {
		writeError(w, r, http.StatusInternalServerError, "save agent token failed")
		return
	}
	s.logAudit(r, "issue_agent_token", "device", id, "生成节点 Agent Token")
	device, _ := s.deviceByID(id)
	writeOK(w, r, map[string]interface{}{"token": token, "device": device, "expiresAt": expiresAt.Format(time.RFC3339), "shownOnce": true})
}

func decodeAgentTokenExpiry(r *http.Request) (time.Time, error) {
	expiresAt := time.Now().UTC().Add(defaultAgentTokenTTL)
	if r.Body == nil {
		return expiresAt, nil
	}
	var payload struct {
		TTLHours  int    `json:"ttlHours"`
		ExpiresAt string `json:"expiresAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		return time.Time{}, errors.New("invalid json")
	}
	if payload.ExpiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(payload.ExpiresAt))
		if err != nil {
			return time.Time{}, errors.New("expiresAt must be RFC3339")
		}
		if !parsed.After(time.Now().UTC()) {
			return time.Time{}, errors.New("expiresAt must be in the future")
		}
		return parsed.UTC(), nil
	}
	if payload.TTLHours > 0 {
		if payload.TTLHours > 24*90 {
			return time.Time{}, errors.New("ttlHours must be 2160 or less")
		}
		expiresAt = time.Now().UTC().Add(time.Duration(payload.TTLHours) * time.Hour)
	}
	return expiresAt, nil
}

func (s *server) heartbeatDevice(r *http.Request, id int) error {
	var payload device
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		return errors.New("invalid json")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	status := payload.Status
	if status == "" {
		status = "online"
	}
	load := firstNonEmpty(payload.Load, "0%")
	inbound := firstNonEmpty(payload.InboundTraffic, "0.00 GiB")
	outbound := firstNonEmpty(payload.OutboundTraffic, "0.00 GiB")
	_, err := s.db.Exec(`UPDATE devices SET status = ?, load = ?, latency_ms = ?, connection_count = ?, inbound_traffic = ?, outbound_traffic = ?, last_heartbeat = ?, last_seen = ?, enabled = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, load, payload.LatencyMs, payload.ConnectionCount, inbound, outbound, now, now, id)
	return err
}

func decodeDevice(r *http.Request) (device, error) {
	var item device
	if r.Body == nil {
		return item, nil
	}
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		return item, errors.New("invalid json")
	}
	item.Name = strings.TrimSpace(item.Name)
	item.GroupName = strings.TrimSpace(item.GroupName)
	item.Type = strings.TrimSpace(item.Type)
	item.Status = strings.TrimSpace(item.Status)
	item.Address = strings.TrimSpace(item.Address)
	item.Region = strings.TrimSpace(item.Region)
	item.Version = strings.TrimSpace(item.Version)
	item.Load = strings.TrimSpace(item.Load)
	item.Remark = strings.TrimSpace(item.Remark)
	if r.Method == http.MethodPost {
		if item.Name == "" {
			return item, errors.New("name is required")
		}
		if item.Type == "" {
			item.Type = "出口"
		}
		if item.Status == "" {
			item.Status = "offline"
		}
		if item.GroupName == "" && item.GroupID > 0 {
			item.GroupName = fmt.Sprintf("#%d", item.GroupID)
		}
		if item.Version == "" {
			item.Version = "edge-0.1.0"
		}
		if item.Load == "" {
			item.Load = "0%"
		}
		if item.InboundTraffic == "" {
			item.InboundTraffic = "0.00 GiB"
		}
		if item.OutboundTraffic == "" {
			item.OutboundTraffic = "0.00 GiB"
		}
		if item.LastSeen == "" {
			item.LastSeen = time.Now().UTC().Format(time.RFC3339)
		}
		item.Enabled = item.Status != "disabled"
	}
	return item, nil
}

func (s *server) listDevices(r *http.Request) ([]device, error) {
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	groupID := atoiDefault(r.URL.Query().Get("groupId"), 0)
	query := deviceSelectSQL + ` WHERE 1=1`
	args := []interface{}{}
	if keyword != "" {
		query += ` AND (name LIKE ? OR address LIKE ? OR region LIKE ? OR remark LIKE ?)`
		like := "%" + keyword + "%"
		args = append(args, like, like, like, like)
	}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	if groupID > 0 {
		query += ` AND group_id = ?`
		args = append(args, groupID)
	}
	query += ` ORDER BY id ASC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []device
	for rows.Next() {
		var item device
		if err := scanDevice(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *server) deviceByID(id int) (device, error) {
	var item device
	err := scanDevice(s.db.QueryRow(deviceSelectSQL+` WHERE id = ?`, id), &item)
	return item, err
}

const deviceSelectSQL = `SELECT id, name, group_id, group_name, type, status, address, region, version, load, latency_ms, connection_count, inbound_traffic, outbound_traffic, last_heartbeat, last_seen, enabled, agent_registered_at, agent_token_expires_at, agent_token_rotated_at, config_version, remark FROM devices`

func scanDevice(scanner sqlScanner, item *device) error {
	return scanner.Scan(&item.ID, &item.Name, &item.GroupID, &item.GroupName, &item.Type, &item.Status, &item.Address, &item.Region, &item.Version, &item.Load, &item.LatencyMs, &item.ConnectionCount, &item.InboundTraffic, &item.OutboundTraffic, &item.LastHeartbeat, &item.LastSeen, &item.Enabled, &item.AgentRegisteredAt, &item.AgentTokenExpiresAt, &item.AgentTokenRotatedAt, &item.ConfigVersion, &item.Remark)
}

func (s *server) recountDeviceGroupOnline() error {
	_, err := s.db.Exec(`UPDATE device_groups SET online_devices = (
		SELECT COUNT(*) FROM devices WHERE devices.group_id = device_groups.id AND devices.status = 'online' AND devices.enabled = 1
	), updated_at = CURRENT_TIMESTAMP`)
	return err
}

func (s *server) bumpAgentConfigVersions() error {
	_, err := s.db.Exec(`UPDATE devices SET config_version = config_version + 1, updated_at = CURRENT_TIMESTAMP WHERE enabled = 1`)
	return err
}

func (s *server) agentRoute(w http.ResponseWriter, r *http.Request, path string) {
	device, err := s.currentAgentDevice(r)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "invalid agent token")
		return
	}
	switch path {
	case "/api/v1/agent/register":
		s.agentRegister(w, r, device)
	case "/api/v1/agent/heartbeat":
		s.agentHeartbeat(w, r, device)
	case "/api/v1/agent/config":
		s.agentConfig(w, r, device)
	case "/api/v1/agent/connections":
		s.agentConnections(w, r, device)
	case "/api/v1/agent/tunnel":
		s.agentTunnel(w, r, device)
	default:
		writeError(w, r, http.StatusNotFound, "agent endpoint not found")
	}
}

func (s *server) currentAgentDevice(r *http.Request) (device, error) {
	token := agentToken(r)
	if token == "" {
		return device{}, errors.New("agent token required")
	}
	var item device
	now := time.Now().UTC().Format(time.RFC3339)
	err := scanDevice(s.db.QueryRow(deviceSelectSQL+` WHERE agent_token_hash = ? AND enabled = 1 AND (agent_token_expires_at = '' OR agent_token_expires_at > ?)`, hashAgentToken(token), now), &item)
	return item, err
}

func agentToken(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	return strings.TrimSpace(r.Header.Get("X-Agent-Token"))
}

func (s *server) agentRegister(w http.ResponseWriter, r *http.Request, item device) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	payload, err := decodeAgentDevicePayload(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	address := firstNonEmpty(payload.Address, item.Address, remoteHost(r.RemoteAddr))
	version := firstNonEmpty(payload.Version, item.Version)
	region := firstNonEmpty(payload.Region, item.Region)
	if _, err := s.db.Exec(`UPDATE devices SET status = 'online', address = ?, region = ?, version = ?, load = ?, latency_ms = ?, connection_count = ?, inbound_traffic = ?, outbound_traffic = ?, last_heartbeat = ?, last_seen = ?, agent_registered_at = CASE WHEN agent_registered_at = '' THEN ? ELSE agent_registered_at END, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		address, region, version, firstNonEmpty(payload.Load, item.Load, "0%"), payload.LatencyMs, payload.ConnectionCount, firstNonEmpty(payload.InboundTraffic, item.InboundTraffic, "0.00 GiB"), firstNonEmpty(payload.OutboundTraffic, item.OutboundTraffic, "0.00 GiB"), now, now, now, item.ID); err != nil {
		writeError(w, r, http.StatusInternalServerError, "register agent failed")
		return
	}
	_ = s.recountDeviceGroupOnline()
	updated, _ := s.deviceByID(item.ID)
	writeOK(w, r, agentEnvelope(updated, map[string]interface{}{"registered": true}))
}

func (s *server) agentHeartbeat(w http.ResponseWriter, r *http.Request, item device) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	payload, err := decodeAgentDevicePayload(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	status := firstNonEmpty(payload.Status, "online")
	if _, err := s.db.Exec(`UPDATE devices SET status = ?, load = ?, latency_ms = ?, connection_count = ?, inbound_traffic = ?, outbound_traffic = ?, last_heartbeat = ?, last_seen = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, firstNonEmpty(payload.Load, item.Load, "0%"), payload.LatencyMs, payload.ConnectionCount, firstNonEmpty(payload.InboundTraffic, item.InboundTraffic, "0.00 GiB"), firstNonEmpty(payload.OutboundTraffic, item.OutboundTraffic, "0.00 GiB"), now, now, item.ID); err != nil {
		writeError(w, r, http.StatusInternalServerError, "heartbeat failed")
		return
	}
	_ = s.recountDeviceGroupOnline()
	updated, _ := s.deviceByID(item.ID)
	writeOK(w, r, agentEnvelope(updated, map[string]interface{}{"nextHeartbeatSec": 30}))
}

func decodeAgentDevicePayload(r *http.Request) (device, error) {
	var payload device
	if r.Body == nil {
		return payload, nil
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		return payload, errors.New("invalid json")
	}
	payload.Status = strings.TrimSpace(payload.Status)
	payload.Address = strings.TrimSpace(payload.Address)
	payload.Region = strings.TrimSpace(payload.Region)
	payload.Version = strings.TrimSpace(payload.Version)
	payload.Load = strings.TrimSpace(payload.Load)
	payload.InboundTraffic = strings.TrimSpace(payload.InboundTraffic)
	payload.OutboundTraffic = strings.TrimSpace(payload.OutboundTraffic)
	return payload, nil
}

func (s *server) agentConfig(w http.ResponseWriter, r *http.Request, item device) {
	if r.Method != http.MethodGet {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	rules, err := s.agentRules(item)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "load agent config failed")
		return
	}
	checks, err := s.agentHealthChecks(item)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "load agent config failed")
		return
	}
	writeOK(w, r, agentEnvelope(item, map[string]interface{}{
		"rules":        rules,
		"healthChecks": checks,
		"scope": map[string]interface{}{
			"deviceId":   item.ID,
			"groupId":    item.GroupID,
			"groupName":  item.GroupName,
			"deviceType": item.Type,
		},
		"generatedAt": time.Now().UTC().Format(time.RFC3339),
	}))
}

func (s *server) agentRules(item device) ([]agentRulePayload, error) {
	rules, err := s.listRules()
	if err != nil {
		return nil, err
	}
	var result []agentRulePayload
	for _, rule := range rules {
		if !rule.Enabled || rule.Status == "已暂停" {
			continue
		}
		role := ""
		entryMatch := rule.EntryGroupID == item.GroupID || (rule.EntryGroupID == 0 && rule.EntryGroupName == item.GroupName)
		exitMatch := rule.ExitGroupID == item.GroupID || (rule.ExitGroupID == 0 && rule.ExitGroupName == item.GroupName)
		if entryMatch {
			role = "entry"
		}
		if exitMatch {
			if role == "entry" {
				role = "entry_exit"
			} else {
				role = "exit"
			}
		}
		if role == "" {
			continue
		}
		mode := agentRuleMode(rule, role)
		exitDevices := []device{}
		if mode == "reverse_tunnel" && (role == "entry" || role == "entry_exit") {
			exitDevices, err = s.devicesByGroup(rule.ExitGroupID, rule.ExitGroupName)
			if err != nil {
				return nil, err
			}
		}
		result = append(result, agentRulePayload{
			Role:        role,
			Mode:        mode,
			Rule:        rule,
			Entry:       agentEntryForRule(rule, role),
			Exit:        agentExitForRule(rule, role),
			Tunnel:      agentTunnelForRule(mode, exitDevices),
			ExitDevices: exitDevices,
		})
	}
	return result, nil
}

func agentRuleMode(rule forwardRule, role string) string {
	if role == "exit" {
		return "exit_only"
	}
	switch normalizeAgentProtocol(rule.Protocol) {
	case "ws", "wss", "reverse", "reverse-tunnel":
		return "reverse_tunnel"
	default:
		return "direct"
	}
}

func agentEntryForRule(rule forwardRule, role string) agentEntryConfig {
	enabled := role == "entry" || role == "entry_exit"
	return agentEntryConfig{
		Enabled:           enabled,
		ListenHost:        rule.ListenHost,
		ListenPort:        rule.ListenPort,
		ListenAddr:        joinHostPortIfPossible(rule.ListenHost, firstPort(rule.ListenPort)),
		Protocol:          rule.Protocol,
		ProxyProtocol:     rule.Proxy,
		ProxyProtocolMode: rule.ProxyProtocolMode,
	}
}

func agentExitForRule(rule forwardRule, role string) agentExitConfig {
	enabled := role == "exit" || role == "entry_exit"
	return agentExitConfig{
		Enabled:           enabled,
		TargetHost:        rule.TargetHost,
		TargetPort:        rule.TargetPort,
		TargetAddr:        joinHostPortIfPossible(rule.TargetHost, firstPort(rule.TargetPort)),
		ProxyProtocol:     rule.Proxy,
		ProxyProtocolMode: rule.ProxyProtocolMode,
	}
}

func agentTunnelForRule(mode string, exitDevices []device) agentTunnelConfig {
	peerIDs := make([]int, 0, len(exitDevices))
	for _, item := range exitDevices {
		peerIDs = append(peerIDs, item.ID)
	}
	return agentTunnelConfig{
		Enabled:       mode == "reverse_tunnel",
		PeerPolicy:    "first_online",
		PeerDeviceIDs: peerIDs,
		ExitDevices:   exitDevices,
	}
}

func normalizeAgentProtocol(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	switch value {
	case "tls-入站", "tls":
		return "tls-passthrough"
	default:
		return value
	}
}

func joinHostPortIfPossible(host string, port string) string {
	port = strings.TrimSpace(port)
	if port == "" {
		return ""
	}
	return net.JoinHostPort(strings.TrimSpace(host), port)
}

func (s *server) devicesByGroup(groupID int, groupName string) ([]device, error) {
	query := deviceSelectSQL + ` WHERE enabled = 1`
	args := []interface{}{}
	if groupID > 0 {
		query += ` AND group_id = ?`
		args = append(args, groupID)
	} else if groupName != "" {
		query += ` AND group_name = ?`
		args = append(args, groupName)
	} else {
		return []device{}, nil
	}
	query += ` ORDER BY CASE status WHEN 'online' THEN 0 ELSE 1 END, id ASC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []device
	for rows.Next() {
		var item device
		if err := scanDevice(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *server) agentHealthChecks(item device) ([]healthCheck, error) {
	rows, err := s.db.Query(healthCheckSelectSQL+` WHERE enabled = 1 AND target_type = 'device' AND target_id = ? ORDER BY id ASC`, item.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var checks []healthCheck
	for rows.Next() {
		var check healthCheck
		if err := scanHealthCheck(rows, &check); err != nil {
			return nil, err
		}
		checks = append(checks, check)
	}
	return checks, rows.Err()
}

func (s *server) agentConnections(w http.ResponseWriter, r *http.Request, item device) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	connections, err := decodeAgentConnections(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	upserted := 0
	for _, conn := range connections {
		if conn.SourceIP == "" {
			continue
		}
		if err := s.upsertAgentConnection(item, conn); err != nil {
			writeError(w, r, http.StatusInternalServerError, "save connection failed")
			return
		}
		upserted++
	}
	_ = s.refreshRuntimeStats()
	writeOK(w, r, agentEnvelope(item, map[string]interface{}{"upserted": upserted}))
}

func (s *server) agentTunnel(w http.ResponseWriter, r *http.Request, item device) {
	if r.Method != http.MethodGet {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !strings.EqualFold(r.Header.Get("Upgrade"), "repleypass-tunnel") {
		writeError(w, r, http.StatusBadRequest, "upgrade required")
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		writeError(w, r, http.StatusInternalServerError, "hijack unsupported")
		return
	}
	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return
	}
	_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: repleypass-tunnel\r\nConnection: Upgrade\r\n\r\n")
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return
	}
	s.tunnels.serve(item.ID, conn)
}

func (s *server) validateTunnelOpen(sourceDeviceID int, frame tunnelFrame) error {
	var payload tunnelOpenPayload
	if len(frame.Payload) == 0 || frame.Payload[0] != '{' {
		return errors.New("tunnel open metadata is required")
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return errors.New("invalid tunnel open metadata")
	}
	payload.TargetAddr = strings.TrimSpace(payload.TargetAddr)
	if payload.RuleID <= 0 {
		return errors.New("tunnel ruleId is required")
	}
	if payload.TargetAddr == "" {
		return errors.New("tunnel targetAddr is required")
	}
	rule, err := s.ruleByID(payload.RuleID)
	if err != nil {
		return errors.New("tunnel rule not found")
	}
	if !rule.Enabled || rule.Status == "已暂停" {
		return errors.New("tunnel rule is disabled")
	}
	source, err := s.deviceByID(sourceDeviceID)
	if err != nil || !source.Enabled {
		return errors.New("source device is disabled")
	}
	target, err := s.deviceByID(int(frame.TargetDeviceID))
	if err != nil || !target.Enabled {
		return errors.New("target device is disabled")
	}
	if !deviceInRuleGroup(source, rule.EntryGroupID, rule.EntryGroupName) {
		return errors.New("source device is not in rule entry group")
	}
	if !deviceInRuleGroup(target, rule.ExitGroupID, rule.ExitGroupName) {
		return errors.New("target device is not in rule exit group")
	}
	if expected, ok := ruleTargetAddress(rule); ok && payload.TargetAddr != expected {
		return errors.New("targetAddr does not match rule target")
	}
	return nil
}

func deviceInRuleGroup(item device, groupID int, groupName string) bool {
	if groupID > 0 {
		return item.GroupID == groupID
	}
	return groupName != "" && item.GroupName == groupName
}

func ruleTargetAddress(rule forwardRule) (string, bool) {
	host := strings.TrimSpace(rule.TargetHost)
	port := firstPort(rule.TargetPort)
	if host == "" || port == "" {
		return "", false
	}
	return net.JoinHostPort(host, port), true
}

func newTunnelHub(validate tunnelValidator) *tunnelHub {
	return &tunnelHub{conns: map[int]*tunnelPeer{}, routes: map[string]int{}, validate: validate}
}

func (h *tunnelHub) serve(deviceID int, conn net.Conn) {
	peer := &tunnelPeer{deviceID: deviceID, conn: conn}
	h.register(peer)
	defer h.unregister(deviceID, peer)
	defer conn.Close()
	log.Printf("agent tunnel connected device=%d", deviceID)
	for {
		frame, err := readTunnelFrame(conn)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("agent tunnel read failed device=%d: %v", deviceID, err)
			}
			return
		}
		if frame.Type == tunnelFramePing {
			frame.SourceDeviceID = uint32(deviceID)
			frame.TargetDeviceID = uint32(deviceID)
			_ = peer.write(frame)
			continue
		}
		frame.SourceDeviceID = uint32(deviceID)
		if frame.TargetDeviceID == 0 {
			log.Printf("drop tunnel frame without target device=%d stream=%d", deviceID, frame.StreamID)
			continue
		}
		if frame.Type == tunnelFrameOpen {
			if err := h.openRoute(deviceID, frame); err != nil {
				log.Printf("reject tunnel open from=%d to=%d stream=%d: %v", deviceID, frame.TargetDeviceID, frame.StreamID, err)
				_ = peer.write(tunnelFrame{Type: tunnelFrameClose, StreamID: frame.StreamID, SourceDeviceID: frame.TargetDeviceID, TargetDeviceID: uint32(deviceID), Payload: []byte(err.Error())})
				continue
			}
		} else if frame.Type == tunnelFrameData || frame.Type == tunnelFrameClose {
			if err := h.validateRoute(deviceID, frame); err != nil {
				log.Printf("drop tunnel frame from=%d to=%d stream=%d: %v", deviceID, frame.TargetDeviceID, frame.StreamID, err)
				_ = peer.write(tunnelFrame{Type: tunnelFrameClose, StreamID: frame.StreamID, SourceDeviceID: frame.TargetDeviceID, TargetDeviceID: uint32(deviceID), Payload: []byte(err.Error())})
				continue
			}
		} else {
			log.Printf("drop unsupported tunnel frame type=%d device=%d stream=%d", frame.Type, deviceID, frame.StreamID)
			continue
		}
		if err := h.forward(frame); err != nil {
			log.Printf("forward tunnel frame failed from=%d to=%d stream=%d: %v", deviceID, frame.TargetDeviceID, frame.StreamID, err)
			_ = peer.write(tunnelFrame{Type: tunnelFrameClose, StreamID: frame.StreamID, SourceDeviceID: frame.TargetDeviceID, TargetDeviceID: uint32(deviceID), Payload: []byte(err.Error())})
			if frame.Type == tunnelFrameOpen {
				h.closeRoute(deviceID, int(frame.TargetDeviceID), frame.StreamID)
			}
			continue
		}
		if frame.Type == tunnelFrameClose {
			h.closeRoute(deviceID, int(frame.TargetDeviceID), frame.StreamID)
		}
	}
}

func (h *tunnelHub) register(peer *tunnelPeer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if old := h.conns[peer.deviceID]; old != nil {
		_ = old.conn.Close()
	}
	h.conns[peer.deviceID] = peer
}

func (h *tunnelHub) unregister(deviceID int, peer *tunnelPeer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conns[deviceID] == peer {
		delete(h.conns, deviceID)
		h.dropDeviceRoutesLocked(deviceID)
	}
	log.Printf("agent tunnel disconnected device=%d", deviceID)
}

func (h *tunnelHub) openRoute(deviceID int, frame tunnelFrame) error {
	if h.validate != nil {
		if err := h.validate(deviceID, frame); err != nil {
			return err
		}
	}
	targetID := int(frame.TargetDeviceID)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conns[targetID] == nil {
		return fmt.Errorf("target device %d tunnel offline", targetID)
	}
	h.routes[tunnelRouteKey(deviceID, frame.StreamID)] = targetID
	h.routes[tunnelRouteKey(targetID, frame.StreamID)] = deviceID
	return nil
}

func (h *tunnelHub) validateRoute(deviceID int, frame tunnelFrame) error {
	h.mu.RLock()
	targetID, ok := h.routes[tunnelRouteKey(deviceID, frame.StreamID)]
	h.mu.RUnlock()
	if !ok {
		return errors.New("unknown tunnel stream")
	}
	if targetID != int(frame.TargetDeviceID) {
		return fmt.Errorf("invalid tunnel route target %d", frame.TargetDeviceID)
	}
	return nil
}

func (h *tunnelHub) closeRoute(deviceID int, targetID int, streamID uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.routes, tunnelRouteKey(deviceID, streamID))
	delete(h.routes, tunnelRouteKey(targetID, streamID))
}

func (h *tunnelHub) dropDeviceRoutesLocked(deviceID int) {
	for key, targetID := range h.routes {
		if strings.HasPrefix(key, fmt.Sprintf("%d:", deviceID)) || targetID == deviceID {
			delete(h.routes, key)
		}
	}
}

func tunnelRouteKey(deviceID int, streamID uint64) string {
	return fmt.Sprintf("%d:%d", deviceID, streamID)
}

func (h *tunnelHub) forward(frame tunnelFrame) error {
	h.mu.RLock()
	peer := h.conns[int(frame.TargetDeviceID)]
	h.mu.RUnlock()
	if peer == nil {
		return fmt.Errorf("target device %d tunnel offline", frame.TargetDeviceID)
	}
	return peer.write(frame)
}

func (p *tunnelPeer) write(frame tunnelFrame) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return writeTunnelFrame(p.conn, frame)
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

func decodeAgentConnections(r *http.Request) ([]agentConnectionReport, error) {
	var payload struct {
		Connections []agentConnectionReport `json:"connections"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, errors.New("invalid json")
	}
	if len(payload.Connections) == 0 {
		return nil, errors.New("connections are required")
	}
	for i := range payload.Connections {
		payload.Connections[i].SourceIP = strings.TrimSpace(payload.Connections[i].SourceIP)
		payload.Connections[i].RuleName = strings.TrimSpace(payload.Connections[i].RuleName)
		payload.Connections[i].Protocol = strings.TrimSpace(payload.Connections[i].Protocol)
		payload.Connections[i].RealIPSource = strings.TrimSpace(payload.Connections[i].RealIPSource)
		payload.Connections[i].Country = strings.TrimSpace(payload.Connections[i].Country)
		payload.Connections[i].UserAgent = strings.TrimSpace(payload.Connections[i].UserAgent)
		payload.Connections[i].Remark = strings.TrimSpace(payload.Connections[i].Remark)
	}
	return payload.Connections, nil
}

func (s *server) upsertAgentConnection(item device, conn agentConnectionReport) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if conn.Protocol == "" {
		conn.Protocol = "TCP"
	}
	if conn.RealIPSource == "" {
		conn.RealIPSource = "connection_log"
	}
	if conn.ConnectionCount <= 0 {
		conn.ConnectionCount = 1
	}
	if conn.RuleName == "" && conn.RuleID > 0 {
		if rule, err := s.ruleByID(conn.RuleID); err == nil {
			conn.RuleName = rule.Name
		}
	}
	var id int
	err := s.db.QueryRow(`SELECT id FROM online_ips WHERE source_ip = ? AND source_port = ? AND rule_id = ? AND entry_device_id = ? AND status = 'active' ORDER BY id DESC LIMIT 1`,
		conn.SourceIP, conn.SourcePort, conn.RuleID, item.ID).Scan(&id)
	if err == nil {
		_, err = s.db.Exec(`UPDATE online_ips SET rule_name = ?, entry_device_name = ?, entry_group_name = ?, protocol = ?, real_ip_source = ?, connection_count = ?, last_active_at = ?, country = ?, user_agent = ?, remark = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			conn.RuleName, item.Name, item.GroupName, conn.Protocol, conn.RealIPSource, conn.ConnectionCount, now, conn.Country, conn.UserAgent, conn.Remark, id)
		return err
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO online_ips (source_ip, source_port, rule_id, rule_name, entry_device_id, entry_device_name, entry_group_name, protocol, real_ip_source, connection_count, first_seen, last_active_at, status, country, user_agent, remark) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?, ?)`,
		conn.SourceIP, conn.SourcePort, conn.RuleID, conn.RuleName, item.ID, item.Name, item.GroupName, conn.Protocol, conn.RealIPSource, conn.ConnectionCount, now, now, conn.Country, conn.UserAgent, conn.Remark)
	return err
}

func agentEnvelope(item device, extra map[string]interface{}) map[string]interface{} {
	data := map[string]interface{}{
		"device":        item,
		"configVersion": item.ConfigVersion,
		"serverTime":    time.Now().UTC().Format(time.RFC3339),
	}
	for key, value := range extra {
		data[key] = value
	}
	return data
}

func remoteHost(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}

func (s *server) onlineIPs(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.createOnlineIP(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	items, err := s.listOnlineIPs(r)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "list online ips failed")
		return
	}
	writeOK(w, r, paginate(r, items))
}

func (s *server) onlineIPRoute(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 {
		writeError(w, r, http.StatusBadRequest, "invalid online ip route")
		return
	}
	id, err := strconv.Atoi(parts[len(parts)-1])
	action := ""
	if err != nil && len(parts) >= 5 {
		action = parts[len(parts)-1]
		id, err = strconv.Atoi(parts[len(parts)-2])
	}
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusBadRequest, "invalid online ip id")
		return
	}
	if action == "close" {
		s.closeOnlineIP(w, r, id)
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := s.onlineIPByID(id)
		if err != nil {
			writeError(w, r, http.StatusNotFound, "online ip not found")
			return
		}
		writeOK(w, r, item)
	case http.MethodPatch, http.MethodPut:
		s.updateOnlineIP(w, r, id)
	case http.MethodDelete:
		if _, err := s.db.Exec(`DELETE FROM online_ips WHERE id = ?`, id); err != nil {
			writeError(w, r, http.StatusInternalServerError, "delete online ip failed")
			return
		}
		_ = s.refreshRuntimeStats()
		writeOK(w, r, map[string]interface{}{"deleted": true, "id": id})
	default:
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) createOnlineIP(w http.ResponseWriter, r *http.Request) {
	item, err := decodeOnlineIP(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.db.Exec(`INSERT INTO online_ips (source_ip, source_port, rule_id, rule_name, entry_device_id, entry_device_name, entry_group_name, protocol, real_ip_source, connection_count, first_seen, last_active_at, status, country, user_agent, remark) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.SourceIP, item.SourcePort, item.RuleID, item.RuleName, item.EntryDeviceID, item.EntryDeviceName, item.EntryGroupName, item.Protocol, item.RealIPSource, item.ConnectionCount, item.FirstSeen, item.LastActiveAt, item.Status, item.Country, item.UserAgent, item.Remark)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "create online ip failed")
		return
	}
	id, _ := result.LastInsertId()
	_ = s.refreshRuntimeStats()
	s.logAudit(r, "create", "online_ip", int(id), "记录在线 IP")
	created, _ := s.onlineIPByID(int(id))
	writeJSON(w, r, http.StatusCreated, response{Success: true, Data: created, RequestID: requestID(r)})
}

func (s *server) updateOnlineIP(w http.ResponseWriter, r *http.Request, id int) {
	current, err := s.onlineIPByID(id)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "online ip not found")
		return
	}
	patch, err := decodeOnlineIP(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if patch.SourceIP != "" {
		current.SourceIP = patch.SourceIP
	}
	if patch.SourcePort != 0 {
		current.SourcePort = patch.SourcePort
	}
	if patch.RuleID != 0 {
		current.RuleID = patch.RuleID
	}
	if patch.RuleName != "" {
		current.RuleName = patch.RuleName
	}
	if patch.EntryDeviceID != 0 {
		current.EntryDeviceID = patch.EntryDeviceID
	}
	if patch.EntryDeviceName != "" {
		current.EntryDeviceName = patch.EntryDeviceName
	}
	if patch.EntryGroupName != "" {
		current.EntryGroupName = patch.EntryGroupName
	}
	if patch.Protocol != "" {
		current.Protocol = patch.Protocol
	}
	if patch.RealIPSource != "" {
		current.RealIPSource = patch.RealIPSource
	}
	if patch.ConnectionCount != 0 {
		current.ConnectionCount = patch.ConnectionCount
	}
	if patch.LastActiveAt != "" {
		current.LastActiveAt = patch.LastActiveAt
	}
	if patch.Status != "" {
		current.Status = patch.Status
	}
	if patch.Country != "" {
		current.Country = patch.Country
	}
	if patch.UserAgent != "" {
		current.UserAgent = patch.UserAgent
	}
	if patch.Remark != "" {
		current.Remark = patch.Remark
	}
	if _, err := s.db.Exec(`UPDATE online_ips SET source_ip = ?, source_port = ?, rule_id = ?, rule_name = ?, entry_device_id = ?, entry_device_name = ?, entry_group_name = ?, protocol = ?, real_ip_source = ?, connection_count = ?, first_seen = ?, last_active_at = ?, status = ?, country = ?, user_agent = ?, remark = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		current.SourceIP, current.SourcePort, current.RuleID, current.RuleName, current.EntryDeviceID, current.EntryDeviceName, current.EntryGroupName, current.Protocol, current.RealIPSource, current.ConnectionCount, current.FirstSeen, current.LastActiveAt, current.Status, current.Country, current.UserAgent, current.Remark, id); err != nil {
		writeError(w, r, http.StatusInternalServerError, "update online ip failed")
		return
	}
	_ = s.refreshRuntimeStats()
	writeOK(w, r, current)
}

func (s *server) closeOnlineIP(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`UPDATE online_ips SET status = 'closed', connection_count = 0, last_active_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, now, id); err != nil {
		writeError(w, r, http.StatusInternalServerError, "close online ip failed")
		return
	}
	_ = s.refreshRuntimeStats()
	s.logAudit(r, "close", "online_ip", id, "关闭在线连接")
	item, err := s.onlineIPByID(id)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "online ip not found")
		return
	}
	writeOK(w, r, item)
}

func decodeOnlineIP(r *http.Request) (onlineIP, error) {
	var item onlineIP
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		return item, errors.New("invalid json")
	}
	item.SourceIP = strings.TrimSpace(item.SourceIP)
	item.RuleName = strings.TrimSpace(item.RuleName)
	item.EntryDeviceName = strings.TrimSpace(item.EntryDeviceName)
	item.EntryGroupName = strings.TrimSpace(item.EntryGroupName)
	item.Protocol = strings.TrimSpace(item.Protocol)
	item.RealIPSource = strings.TrimSpace(item.RealIPSource)
	item.Status = strings.TrimSpace(item.Status)
	item.Country = strings.TrimSpace(item.Country)
	item.UserAgent = strings.TrimSpace(item.UserAgent)
	item.Remark = strings.TrimSpace(item.Remark)
	if r.Method == http.MethodPost {
		if item.SourceIP == "" {
			return item, errors.New("sourceIp is required")
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if item.FirstSeen == "" {
			item.FirstSeen = now
		}
		if item.LastActiveAt == "" {
			item.LastActiveAt = now
		}
		if item.Protocol == "" {
			item.Protocol = "TCP"
		}
		if item.RealIPSource == "" {
			item.RealIPSource = "connection_log"
		}
		if item.ConnectionCount == 0 {
			item.ConnectionCount = 1
		}
		if item.Status == "" {
			item.Status = "active"
		}
	}
	return item, nil
}

func (s *server) listOnlineIPs(r *http.Request) ([]onlineIP, error) {
	query := onlineIPSelectSQL + ` WHERE 1=1`
	args := []interface{}{}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if keyword != "" {
		query += ` AND (source_ip LIKE ? OR rule_name LIKE ? OR entry_device_name LIKE ? OR country LIKE ?)`
		like := "%" + keyword + "%"
		args = append(args, like, like, like, like)
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	if ruleID := atoiDefault(r.URL.Query().Get("ruleId"), 0); ruleID > 0 {
		query += ` AND rule_id = ?`
		args = append(args, ruleID)
	}
	if deviceID := atoiDefault(r.URL.Query().Get("entryDeviceId"), 0); deviceID > 0 {
		query += ` AND entry_device_id = ?`
		args = append(args, deviceID)
	}
	query += ` ORDER BY CASE status WHEN 'active' THEN 0 ELSE 1 END, last_active_at DESC, id DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []onlineIP
	for rows.Next() {
		var item onlineIP
		if err := scanOnlineIP(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *server) onlineIPByID(id int) (onlineIP, error) {
	var item onlineIP
	err := scanOnlineIP(s.db.QueryRow(onlineIPSelectSQL+` WHERE id = ?`, id), &item)
	return item, err
}

const onlineIPSelectSQL = `SELECT id, source_ip, source_port, rule_id, rule_name, entry_device_id, entry_device_name, entry_group_name, protocol, real_ip_source, connection_count, first_seen, last_active_at, status, country, user_agent, remark FROM online_ips`

func scanOnlineIP(scanner sqlScanner, item *onlineIP) error {
	return scanner.Scan(&item.ID, &item.SourceIP, &item.SourcePort, &item.RuleID, &item.RuleName, &item.EntryDeviceID, &item.EntryDeviceName, &item.EntryGroupName, &item.Protocol, &item.RealIPSource, &item.ConnectionCount, &item.FirstSeen, &item.LastActiveAt, &item.Status, &item.Country, &item.UserAgent, &item.Remark)
}

func (s *server) healthChecks(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.createHealthCheck(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	items, err := s.listHealthChecks(r)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "list health checks failed")
		return
	}
	writeOK(w, r, paginate(r, items))
}

func (s *server) healthCheckRoute(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 {
		writeError(w, r, http.StatusBadRequest, "invalid health check route")
		return
	}
	id, err := strconv.Atoi(parts[len(parts)-1])
	action := ""
	if err != nil && len(parts) >= 5 {
		action = parts[len(parts)-1]
		id, err = strconv.Atoi(parts[len(parts)-2])
	}
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusBadRequest, "invalid health check id")
		return
	}
	switch action {
	case "":
		s.healthCheckResource(w, r, id)
	case "run":
		s.runHealthCheck(w, r, id)
	case "results":
		s.healthCheckResults(w, r, id)
	default:
		writeError(w, r, http.StatusNotFound, "health check action not found")
	}
}

func (s *server) healthCheckResource(w http.ResponseWriter, r *http.Request, id int) {
	switch r.Method {
	case http.MethodGet:
		item, err := s.healthCheckByID(id)
		if err != nil {
			writeError(w, r, http.StatusNotFound, "health check not found")
			return
		}
		writeOK(w, r, item)
	case http.MethodPatch, http.MethodPut:
		s.updateHealthCheck(w, r, id)
	case http.MethodDelete:
		if _, err := s.db.Exec(`DELETE FROM health_checks WHERE id = ?`, id); err != nil {
			writeError(w, r, http.StatusInternalServerError, "delete health check failed")
			return
		}
		_, _ = s.db.Exec(`DELETE FROM health_check_results WHERE check_id = ?`, id)
		s.logAudit(r, "delete", "health_check", id, "删除健康检查")
		writeOK(w, r, map[string]interface{}{"deleted": true, "id": id})
	default:
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) createHealthCheck(w http.ResponseWriter, r *http.Request) {
	item, err := decodeHealthCheck(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.db.Exec(`INSERT INTO health_checks (name, target_type, target_id, target_name, protocol, host, port, path, interval_sec, timeout_sec, status, last_latency_ms, last_error, last_checked_at, enabled, remark) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.Name, item.TargetType, item.TargetID, item.TargetName, item.Protocol, item.Host, item.Port, item.Path, item.IntervalSec, item.TimeoutSec, item.Status, item.LastLatencyMs, item.LastError, item.LastCheckedAt, item.Enabled, item.Remark)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "create health check failed")
		return
	}
	id, _ := result.LastInsertId()
	s.logAudit(r, "create", "health_check", int(id), "创建健康检查")
	created, _ := s.healthCheckByID(int(id))
	writeJSON(w, r, http.StatusCreated, response{Success: true, Data: created, RequestID: requestID(r)})
}

func (s *server) updateHealthCheck(w http.ResponseWriter, r *http.Request, id int) {
	current, err := s.healthCheckByID(id)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "health check not found")
		return
	}
	patch, err := decodeHealthCheck(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if patch.Name != "" {
		current.Name = patch.Name
	}
	if patch.TargetType != "" {
		current.TargetType = patch.TargetType
	}
	if patch.TargetID != 0 {
		current.TargetID = patch.TargetID
	}
	if patch.TargetName != "" {
		current.TargetName = patch.TargetName
	}
	if patch.Protocol != "" {
		current.Protocol = patch.Protocol
	}
	if patch.Host != "" {
		current.Host = patch.Host
	}
	if patch.Port != 0 {
		current.Port = patch.Port
	}
	if patch.Path != "" {
		current.Path = patch.Path
	}
	if patch.IntervalSec != 0 {
		current.IntervalSec = patch.IntervalSec
	}
	if patch.TimeoutSec != 0 {
		current.TimeoutSec = patch.TimeoutSec
	}
	if patch.Status != "" {
		current.Status = patch.Status
	}
	if patch.LastLatencyMs != 0 {
		current.LastLatencyMs = patch.LastLatencyMs
	}
	if patch.LastError != "" {
		current.LastError = patch.LastError
	}
	if patch.LastCheckedAt != "" {
		current.LastCheckedAt = patch.LastCheckedAt
	}
	if patch.Remark != "" {
		current.Remark = patch.Remark
	}
	current.Enabled = current.Status != "disabled"
	if _, err := s.db.Exec(`UPDATE health_checks SET name = ?, target_type = ?, target_id = ?, target_name = ?, protocol = ?, host = ?, port = ?, path = ?, interval_sec = ?, timeout_sec = ?, status = ?, last_latency_ms = ?, last_error = ?, last_checked_at = ?, enabled = ?, remark = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		current.Name, current.TargetType, current.TargetID, current.TargetName, current.Protocol, current.Host, current.Port, current.Path, current.IntervalSec, current.TimeoutSec, current.Status, current.LastLatencyMs, current.LastError, current.LastCheckedAt, current.Enabled, current.Remark, id); err != nil {
		writeError(w, r, http.StatusInternalServerError, "update health check failed")
		return
	}
	s.logAudit(r, "update", "health_check", id, "修改健康检查")
	writeOK(w, r, current)
}

func (s *server) runHealthCheck(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	check, err := s.healthCheckByID(id)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "health check not found")
		return
	}
	updated, err := s.performAndRecordHealthCheck(r.Context(), check, "manual")
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "update health check failed")
		return
	}
	s.logAudit(r, "run", "health_check", id, "执行健康检查")
	writeOK(w, r, updated)
}

func (s *server) startHealthCheckScheduler(ctx context.Context) {
	interval := healthSchedulerInterval()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.runDueHealthChecks(ctx)
			timer.Reset(interval)
		}
	}
}

func healthSchedulerInterval() time.Duration {
	seconds, err := strconv.Atoi(getenv("REPLEYPASS_HEALTH_SCHEDULER_SEC", "30"))
	if err != nil || seconds < 5 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func (s *server) runDueHealthChecks(ctx context.Context) {
	s.healthCheckMu.Lock()
	defer s.healthCheckMu.Unlock()

	rows, err := s.db.Query(healthCheckSelectSQL + ` WHERE enabled = 1 ORDER BY id ASC`)
	if err != nil {
		log.Printf("list due health checks failed: %v", err)
		return
	}
	defer rows.Close()

	now := time.Now().UTC()
	var checks []healthCheck
	for rows.Next() {
		var item healthCheck
		if err := scanHealthCheck(rows, &item); err != nil {
			log.Printf("scan due health check failed: %v", err)
			return
		}
		if healthCheckDue(item, now) {
			checks = append(checks, item)
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("iterate due health checks failed: %v", err)
		return
	}
	for _, check := range checks {
		if _, err := s.performAndRecordHealthCheck(ctx, check, "scheduler"); err != nil {
			log.Printf("run health check id=%d failed: %v", check.ID, err)
		}
	}
}

func healthCheckDue(check healthCheck, now time.Time) bool {
	if !check.Enabled || check.Status == "disabled" {
		return false
	}
	if check.IntervalSec <= 0 {
		check.IntervalSec = 60
	}
	if check.LastCheckedAt == "" {
		return true
	}
	last, err := time.Parse(time.RFC3339, check.LastCheckedAt)
	if err != nil {
		return true
	}
	return now.Sub(last) >= time.Duration(check.IntervalSec)*time.Second
}

func (s *server) performAndRecordHealthCheck(ctx context.Context, check healthCheck, source string) (healthCheck, error) {
	status, latency, reason := executeHealthCheck(ctx, check)
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`INSERT INTO health_check_results (check_id, status, latency_ms, failure_reason, checked_at) VALUES (?, ?, ?, ?, ?)`, check.ID, status, latency, reason, now); err != nil {
		return check, err
	}
	if _, err := s.db.Exec(`UPDATE health_checks SET status = ?, last_latency_ms = ?, last_error = ?, last_checked_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, latency, reason, now, check.ID); err != nil {
		return check, err
	}
	if statusChangedToAlert(check.Status, status) {
		message := fmt.Sprintf("健康检查%s：%s", healthStatusLabel(status), reason)
		s.logSystemAudit(source, "health_check_alert", "health_check", check.ID, message)
	}
	updated, err := s.healthCheckByID(check.ID)
	if err != nil {
		return check, err
	}
	return updated, nil
}

func executeHealthCheck(ctx context.Context, check healthCheck) (string, int, string) {
	if !check.Enabled || check.Status == "disabled" {
		return "disabled", 0, "探针已禁用"
	}
	timeout := time.Duration(check.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	started := time.Now()
	protocol := strings.ToLower(strings.TrimSpace(check.Protocol))
	switch protocol {
	case "http", "https":
		return executeHTTPHealthCheck(ctx, check, protocol, timeout, started)
	default:
		return executeTCPHealthCheck(ctx, check, timeout, started)
	}
}

func executeTCPHealthCheck(ctx context.Context, check healthCheck, timeout time.Duration, started time.Time) (string, int, string) {
	port := check.Port
	if port <= 0 {
		return "failed", 0, "TCP 探针缺少端口"
	}
	address := net.JoinHostPort(check.Host, strconv.Itoa(port))
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return "failed", elapsedMs(started), healthProbeError(err, "TCP 探针连接失败")
	}
	_ = conn.Close()
	latency := elapsedMs(started)
	if latency >= int(timeout.Milliseconds()*8/10) {
		return "warning", latency, "TCP 延迟接近超时"
	}
	return "healthy", latency, ""
}

func executeHTTPHealthCheck(ctx context.Context, check healthCheck, protocol string, timeout time.Duration, started time.Time) (string, int, string) {
	targetURL, err := healthCheckURL(check, protocol)
	if err != nil {
		return "failed", 0, err.Error()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "failed", 0, err.Error()
	}
	req.Header.Set("User-Agent", "RepleyPass-HealthCheck/0.1")
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
	resp, err := client.Do(req)
	latency := elapsedMs(started)
	if err != nil {
		return "failed", latency, healthProbeError(err, "HTTP 探针请求失败")
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		if latency >= int(timeout.Milliseconds()*8/10) {
			return "warning", latency, "HTTP 延迟接近超时"
		}
		return "healthy", latency, ""
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return "warning", latency, fmt.Sprintf("HTTP 状态码 %d", resp.StatusCode)
	}
	return "failed", latency, fmt.Sprintf("HTTP 状态码 %d", resp.StatusCode)
}

func healthCheckURL(check healthCheck, protocol string) (string, error) {
	host := strings.TrimSpace(check.Host)
	if host == "" {
		return "", errors.New("HTTP 探针缺少主机")
	}
	if check.Port > 0 {
		host = net.JoinHostPort(host, strconv.Itoa(check.Port))
	}
	path := strings.TrimSpace(check.Path)
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u := url.URL{Scheme: protocol, Host: host, Path: path}
	return u.String(), nil
}

func healthProbeError(err error, fallback string) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return "探针请求超时"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "探针主机解析失败"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "探针请求超时"
	}
	return fallback
}

func elapsedMs(started time.Time) int {
	ms := int(time.Since(started).Milliseconds())
	if ms < 1 {
		return 1
	}
	return ms
}

func statusChangedToAlert(previous string, current string) bool {
	return previous != current && (current == "failed" || current == "warning")
}

func healthStatusLabel(status string) string {
	switch status {
	case "failed":
		return "失败"
	case "warning":
		return "告警"
	case "healthy":
		return "恢复"
	default:
		return status
	}
}

func (s *server) healthCheckResults(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodGet {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	rows, err := s.db.Query(`SELECT id, check_id, status, latency_ms, failure_reason, checked_at FROM health_check_results WHERE check_id = ? ORDER BY checked_at DESC, id DESC`, id)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "list health check results failed")
		return
	}
	defer rows.Close()
	var items []healthCheckResult
	for rows.Next() {
		var item healthCheckResult
		if err := rows.Scan(&item.ID, &item.CheckID, &item.Status, &item.LatencyMs, &item.FailureReason, &item.CheckedAt); err != nil {
			writeError(w, r, http.StatusInternalServerError, "list health check results failed")
			return
		}
		items = append(items, item)
	}
	writeOK(w, r, paginate(r, items))
}

func decodeHealthCheck(r *http.Request) (healthCheck, error) {
	var item healthCheck
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		return item, errors.New("invalid json")
	}
	item.Name = strings.TrimSpace(item.Name)
	item.TargetType = strings.TrimSpace(item.TargetType)
	item.TargetName = strings.TrimSpace(item.TargetName)
	item.Protocol = strings.TrimSpace(item.Protocol)
	item.Host = strings.TrimSpace(item.Host)
	item.Path = strings.TrimSpace(item.Path)
	item.Status = strings.TrimSpace(item.Status)
	item.LastError = strings.TrimSpace(item.LastError)
	item.Remark = strings.TrimSpace(item.Remark)
	if r.Method == http.MethodPost {
		if item.Name == "" || item.Host == "" {
			return item, errors.New("name and host are required")
		}
		if item.TargetType == "" {
			item.TargetType = "device"
		}
		if item.Protocol == "" {
			item.Protocol = "tcp"
		}
		if item.IntervalSec == 0 {
			item.IntervalSec = 60
		}
		if item.TimeoutSec == 0 {
			item.TimeoutSec = 5
		}
		if item.Status == "" {
			item.Status = "unknown"
		}
		item.Enabled = item.Status != "disabled"
	}
	return item, nil
}

func (s *server) listHealthChecks(r *http.Request) ([]healthCheck, error) {
	query := healthCheckSelectSQL + ` WHERE 1=1`
	args := []interface{}{}
	if keyword := strings.TrimSpace(r.URL.Query().Get("keyword")); keyword != "" {
		query += ` AND (name LIKE ? OR target_name LIKE ? OR host LIKE ? OR remark LIKE ?)`
		like := "%" + keyword + "%"
		args = append(args, like, like, like, like)
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	if targetType := strings.TrimSpace(r.URL.Query().Get("targetType")); targetType != "" {
		query += ` AND target_type = ?`
		args = append(args, targetType)
	}
	query += ` ORDER BY CASE status WHEN 'failed' THEN 0 WHEN 'warning' THEN 1 WHEN 'unknown' THEN 2 ELSE 3 END, id ASC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []healthCheck
	for rows.Next() {
		var item healthCheck
		if err := scanHealthCheck(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *server) healthCheckByID(id int) (healthCheck, error) {
	var item healthCheck
	err := scanHealthCheck(s.db.QueryRow(healthCheckSelectSQL+` WHERE id = ?`, id), &item)
	return item, err
}

const healthCheckSelectSQL = `SELECT id, name, target_type, target_id, target_name, protocol, host, port, path, interval_sec, timeout_sec, status, last_latency_ms, last_error, last_checked_at, enabled, remark FROM health_checks`

func scanHealthCheck(scanner sqlScanner, item *healthCheck) error {
	return scanner.Scan(&item.ID, &item.Name, &item.TargetType, &item.TargetID, &item.TargetName, &item.Protocol, &item.Host, &item.Port, &item.Path, &item.IntervalSec, &item.TimeoutSec, &item.Status, &item.LastLatencyMs, &item.LastError, &item.LastCheckedAt, &item.Enabled, &item.Remark)
}

func (s *server) users(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`SELECT id, id, username, role, user_type, user_group, plan, plan_expire, traffic_used, traffic_total, max_rules, rate_limit, connection_limit, wallet_balance, telegram_linked, auto_renew_enabled, remark FROM users ORDER BY id ASC`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "list users failed")
		return
	}
	defer rows.Close()
	var items []user
	for rows.Next() {
		var item user
		if err := rows.Scan(&item.ID, &item.UID, &item.Username, &item.Role, &item.UserType, &item.UserGroup, &item.Plan, &item.PlanExpire, &item.TrafficUsed, &item.TrafficTotal, &item.MaxRules, &item.RateLimit, &item.ConnectionLimit, &item.WalletBalance, &item.TelegramLinked, &item.AutoRenewEnabled, &item.Remark); err != nil {
			writeError(w, r, http.StatusInternalServerError, "list users failed")
			return
		}
		items = append(items, item)
	}
	writeOK(w, r, paginate(r, items))
}

func (s *server) userGroups(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`SELECT id, sort, name, user_count FROM user_groups ORDER BY sort ASC, id ASC`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "list user groups failed")
		return
	}
	defer rows.Close()
	var items []userGroup
	for rows.Next() {
		var item userGroup
		if err := rows.Scan(&item.ID, &item.Sort, &item.Name, &item.UserCount); err != nil {
			writeError(w, r, http.StatusInternalServerError, "list user groups failed")
			return
		}
		items = append(items, item)
	}
	writeOK(w, r, items)
}

func (s *server) plans(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`SELECT id, sort, name, type, user_group, traffic, rule_limit, price, hidden, description FROM plans ORDER BY sort ASC, id ASC`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "list plans failed")
		return
	}
	defer rows.Close()
	var items []shopPlan
	for rows.Next() {
		var item shopPlan
		if err := rows.Scan(&item.ID, &item.Sort, &item.Name, &item.Type, &item.UserGroup, &item.Traffic, &item.RuleLimit, &item.Price, &item.Hidden, &item.Description); err != nil {
			writeError(w, r, http.StatusInternalServerError, "list plans failed")
			return
		}
		items = append(items, item)
	}
	writeOK(w, r, items)
}

func (s *server) orders(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`SELECT id, username, created_at, paid_at, info, amount, type, status, commission FROM orders ORDER BY created_at DESC`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "list orders failed")
		return
	}
	defer rows.Close()
	var items []order
	for rows.Next() {
		var item order
		if err := rows.Scan(&item.ID, &item.User, &item.CreatedAt, &item.PaidAt, &item.Info, &item.Amount, &item.Type, &item.Status, &item.Commission); err != nil {
			writeError(w, r, http.StatusInternalServerError, "list orders failed")
			return
		}
		items = append(items, item)
	}
	writeOK(w, r, paginate(r, items))
}

func (s *server) redeemCodes(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`SELECT id, code, plan, discount, remain FROM redeem_codes ORDER BY id ASC`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "list redeem codes failed")
		return
	}
	defer rows.Close()
	var items []redeemCode
	for rows.Next() {
		var item redeemCode
		if err := rows.Scan(&item.ID, &item.Code, &item.Plan, &item.Discount, &item.Remain); err != nil {
			writeError(w, r, http.StatusInternalServerError, "list redeem codes failed")
			return
		}
		items = append(items, item)
	}
	writeOK(w, r, paginate(r, items))
}

func (s *server) auditLogs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`SELECT id, actor, action, resource, resource_id, message, created_at FROM audit_logs ORDER BY id DESC LIMIT 200`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "list audit logs failed")
		return
	}
	defer rows.Close()
	var items []auditLog
	for rows.Next() {
		var item auditLog
		if err := rows.Scan(&item.ID, &item.Actor, &item.Action, &item.Resource, &item.ResourceID, &item.Message, &item.CreatedAt); err != nil {
			writeError(w, r, http.StatusInternalServerError, "list audit logs failed")
			return
		}
		items = append(items, item)
	}
	writeOK(w, r, paginate(r, items))
}

func (s *server) logAudit(r *http.Request, action string, resource string, resourceID int, message string) {
	actor := "system"
	if u, ok := s.currentUser(r); ok && u.Username != "" {
		actor = u.Username
	}
	s.logAuditActor(actor, action, resource, resourceID, message)
}

func (s *server) logSystemAudit(actor string, action string, resource string, resourceID int, message string) {
	if actor == "" {
		actor = "system"
	}
	s.logAuditActor(actor, action, resource, resourceID, message)
}

func (s *server) logAuditActor(actor string, action string, resource string, resourceID int, message string) {
	if _, err := s.db.Exec(`INSERT INTO audit_logs (actor, action, resource, resource_id, message) VALUES (?, ?, ?, ?, ?)`, actor, action, resource, resourceID, message); err != nil {
		log.Printf("audit log failed: %v", err)
	}
}

func (s *server) statistic(w http.ResponseWriter, r *http.Request) {
	users, _ := s.countRows("users")
	onlineNodes, _ := s.countWhere("devices", "status = 'online' AND enabled = 1")
	failedHealthChecks, _ := s.countWhere("health_checks", "status IN ('failed', 'warning') AND enabled = 1")
	writeOK(w, r, map[string]interface{}{
		"todayRecharge":      "0.00CNY",
		"yesterdayRecharge":  "0.00CNY",
		"monthRecharge":      "0.00CNY",
		"lastMonthRecharge":  "0.00CNY",
		"todayTraffic":       "0.00GiB",
		"yesterdayTraffic":   "0.00GiB",
		"totalUsers":         users,
		"onlineNodes":        onlineNodes,
		"failedHealthChecks": failedHealthChecks,
		"topUsers":           []map[string]interface{}{{"username": "admin", "traffic": "0.00 GiB"}},
		"topNodes":           []map[string]interface{}{{"name": "香港入口 A", "traffic": "0.00 GiB"}},
	})
}

func (s *server) dashboardOverview(w http.ResponseWriter, r *http.Request) {
	onlineNodes, _ := s.countWhere("devices", "status = 'online' AND enabled = 1")
	alertNodes, _ := s.countWhere("devices", "status = 'alert'")
	failedHealthChecks, _ := s.countWhere("health_checks", "status IN ('failed', 'warning') AND enabled = 1")
	var activeConnections int
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(connection_count), 0) FROM online_ips WHERE status = 'active'`).Scan(&activeConnections)
	writeOK(w, r, map[string]interface{}{
		"onlineNodes":        onlineNodes,
		"activeConnections":  activeConnections,
		"todayTraffic":       "0.00 GiB",
		"realIPRate":         "100%",
		"alerts":             alertNodes + failedHealthChecks,
		"failedHealthChecks": failedHealthChecks,
		"syncFailures":       0,
	})
}

func (s *server) nodeStatus(w http.ResponseWriter, r *http.Request) {
	devices, err := s.listDevices(r)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "list node status failed")
		return
	}
	var nodes []device
	for _, item := range devices {
		nodes = append(nodes, item)
	}
	writeOK(w, r, nodes)
}

func (s *server) forwardFolders(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`SELECT rule_group, COUNT(*) FROM forward_rules GROUP BY rule_group ORDER BY rule_group ASC`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "list folders failed")
		return
	}
	defer rows.Close()
	items := []map[string]interface{}{}
	id := 0
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			writeError(w, r, http.StatusInternalServerError, "list folders failed")
			return
		}
		items = append(items, map[string]interface{}{"id": id, "name": name, "count": count})
		id++
	}
	writeOK(w, r, items)
}

func (s *server) deviceGroupFolders(w http.ResponseWriter, r *http.Request) {
	count, _ := s.countRows("device_groups")
	writeOK(w, r, []map[string]interface{}{{"id": 0, "name": "未分组", "count": count}})
}

func (s *server) topology(w http.ResponseWriter, r *http.Request) {
	groups, err := s.listDeviceGroups(false)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "list topology failed")
		return
	}
	rules, err := s.listRules()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "list topology failed")
		return
	}
	var entries []deviceGroup
	var exits []deviceGroup
	for _, group := range groups {
		if group.Type == "入口" {
			entries = append(entries, group)
		} else {
			exits = append(exits, group)
		}
	}
	devices, _ := s.listDevices(r)
	writeOK(w, r, map[string]interface{}{"entries": entries, "exits": exits, "rules": rules, "devices": devices})
}

func (s *server) countRows(table string) (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
	return count, err
}

func (s *server) countWhere(table string, where string) (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM " + table + " WHERE " + where).Scan(&count)
	return count, err
}

func (s *server) refreshRuntimeStats() error {
	return refreshRuntimeStats(s.db)
}

func refreshRuntimeStats(db *sql.DB) error {
	if _, err := db.Exec(`UPDATE forward_rules SET
		current_connections = COALESCE((SELECT SUM(connection_count) FROM online_ips WHERE online_ips.rule_id = forward_rules.id AND status = 'active'), 0),
		last_hit_at = COALESCE((SELECT MAX(last_active_at) FROM online_ips WHERE online_ips.rule_id = forward_rules.id AND status = 'active'), last_hit_at),
		updated_at = CURRENT_TIMESTAMP`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE devices SET
		connection_count = COALESCE((SELECT SUM(connection_count) FROM online_ips WHERE online_ips.entry_device_id = devices.id AND status = 'active'), 0),
		updated_at = CURRENT_TIMESTAMP`); err != nil {
		return err
	}
	return nil
}

func paginate[T any](r *http.Request, items []T) pageData {
	page := atoiDefault(r.URL.Query().Get("page"), 1)
	size := atoiDefault(firstNonEmpty(r.URL.Query().Get("size"), r.URL.Query().Get("pageSize")), 10)
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	start := (page - 1) * size
	if start > len(items) {
		start = len(items)
	}
	end := start + size
	if end > len(items) {
		end = len(items)
	}
	return pageData{Items: items[start:end], Total: len(items), Page: page, PageSize: size}
}

func writeOK(w http.ResponseWriter, r *http.Request, data interface{}) {
	writeJSON(w, r, http.StatusOK, response{Success: true, Data: data, RequestID: requestID(r)})
}

func writeError(w http.ResponseWriter, r *http.Request, status int, message string) {
	writeJSON(w, r, status, response{Success: false, Data: nil, Message: message, RequestID: requestID(r)})
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, payload response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Id")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), "request_started_at", start)))
		log.Printf("%s %s %s", r.Method, r.URL.RequestURI(), time.Since(start).Round(time.Millisecond))
	})
}

func requestID(r *http.Request) string {
	if id := r.Header.Get("X-Request-Id"); id != "" {
		return id
	}
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

func pathID(path string) int {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return 0
	}
	id, _ := strconv.Atoi(parts[len(parts)-1])
	return id
}

func hashPassword(password string) string {
	sum := sha256.Sum256([]byte("repleypass:" + password))
	return hex.EncodeToString(sum[:])
}

func hashAgentToken(token string) string {
	sum := sha256.Sum256([]byte("repleypass-agent:" + token))
	return hex.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func atoiDefault(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstPort(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	return strings.TrimSpace(parts[0])
}

func splitEndpoint(value string) (string, string) {
	parts := strings.Split(value, ":")
	if len(parts) == 0 {
		return strings.TrimSpace(value), ""
	}
	name := strings.TrimSpace(parts[0])
	port := ""
	if len(parts) > 1 {
		port = strings.TrimSpace(parts[len(parts)-1])
	}
	return name, port
}

func composeEndpoint(name string, port string) string {
	name = strings.TrimSpace(name)
	port = strings.TrimSpace(port)
	if name == "" {
		return port
	}
	if port == "" {
		return name
	}
	return name + " : " + port
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
