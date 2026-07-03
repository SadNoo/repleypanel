# RepleyPass API 草案

## 基础约定

- 参考站真实 Base URL：`/api/v1`
- RepleyPass 建议 Base URL：`/api/v1`
- 数据格式：JSON
- 鉴权：第一阶段待定，可先使用 Cookie Session 或 Bearer Token。
- 时间格式：ISO 8601。
- 时区展示：默认按 UTC+8 展示。

参考站已观察接口见：

- `docs/reference-api-observed.md`

以下草案按 RepleyPass 自身需要设计，命名可在实现时选择：

- 贴近参考站：`/api/v1/user/forward`、`/api/v1/admin/devicegroup`。
- 资源化命名：`/api/v1/rules`、`/api/v1/device-groups`。
- 推荐做法：后端内部资源化，外部可提供 nyanpass 风格兼容路由。

## Auth

### POST `/api/v1/auth/login`

请求：

```json
{
  "username": "admin",
  "password": "<admin-password>"
}
```

响应：

```json
{
  "success": true,
  "data": {
    "user": {
      "id": "u_1",
      "username": "admin",
      "role": "super_admin"
    },
    "token": "<optional-session-token>"
  },
  "message": "",
  "requestId": "req_xxx"
}
```

### POST `/api/v1/auth/logout`

### GET `/api/v1/auth/me`

## Dashboard

### GET `/api/v1/dashboard/overview`

返回：

- 在线节点数。
- 活跃连接数。
- 今日流量。
- 真实 IP 捕获率。
- 告警数。
- 最近事件。
- 重点规则。

### GET `/api/v1/dashboard/topology`

返回入口、策略、出口的拓扑关系和状态。

## Devices

### GET `/api/v1/devices`

查询参数：

- `keyword`
- `status`
- `region`
- `page`
- `pageSize`

### POST `/api/v1/devices`

### GET `/api/v1/devices/:id`

### PATCH `/api/v1/devices/:id`

### DELETE `/api/v1/devices/:id`

### POST `/api/v1/devices/:id/disable`

### POST `/api/v1/devices/:id/enable`

### POST `/api/v1/devices/:id/agent-token`

管理员生成节点 Agent Token。明文 token 只在本次响应返回，数据库仅保存哈希、过期时间和轮换时间。重新签发会覆盖旧 token，旧 token 立即失效。

请求：

```json
{
  "ttlHours": 720,
  "expiresAt": "<optional-rfc3339-time>"
}
```

响应：

```json
{
  "success": true,
  "data": {
    "token": "<agent-token>",
    "expiresAt": "<rfc3339-time>",
    "shownOnce": true
  }
}
```

## Node Agent

Agent 接口使用设备 token 鉴权：

```text
Authorization: Bearer <agent_token>
```

### POST `/api/v1/agent/register`

节点首次接入或重连时调用，会上报版本、地址、地区和基础指标，并把设备标记为在线。

请求：

```json
{
  "version": "edge-0.1.0",
  "address": "node.example.com:20000",
  "region": "HK",
  "load": "8%",
  "latencyMs": 12,
  "connectionCount": 0
}
```

### POST `/api/v1/agent/heartbeat`

节点周期心跳和指标上报。

### GET `/api/v1/agent/config`

节点拉取当前配置，返回设备信息、`configVersion`、与该设备组相关的规则，以及绑定到该设备的健康检查配置。

控制面只返回当前设备所在入口/出口组相关的规则；纯出口角色不会收到无关出口候选设备列表。响应会包含当前节点配置作用域：

```json
{
  "configVersion": 1,
  "scope": {
    "deviceId": 1,
    "groupId": 1,
    "groupName": "entry-group",
    "deviceType": "entry"
  },
  "rules": []
}
```

### POST `/api/v1/agent/connections`

节点上报在线连接和真实 IP 识别结果。

请求：

```json
{
  "connections": [
    {
      "sourceIp": "client.example.com",
      "sourcePort": 51234,
      "ruleId": 1,
      "protocol": "TCP",
      "realIpSource": "proxy_protocol_v2",
      "connectionCount": 1,
      "country": "HK"
    }
  ]
}
```

### GET `/api/v1/agent/tunnel`

节点建立控制面中继隧道，使用 HTTP Upgrade：

```text
Connection: Upgrade
Upgrade: repleypass-tunnel
Authorization: Bearer <agent_token>
```

升级后使用 RepleyPass 二进制帧进行 stream 多路复用。tunnel open 必须携带 `ruleId` 与 `targetAddr` 元数据；控制面会校验来源设备属于入口组、目标设备属于出口组、规则启用且目标地址匹配规则后才允许转发。

## Device Groups

### GET `/api/v1/device-groups`

查询参数：

- `keyword`
- `type`：`entry` 或 `exit`
- `status`
- `page`
- `pageSize`

### POST `/api/v1/device-groups`

### GET `/api/v1/device-groups/:id`

### PATCH `/api/v1/device-groups/:id`

### DELETE `/api/v1/device-groups/:id`

### PATCH `/api/v1/device-groups/:id/config`

用于高级 JSON 配置编辑。

## Relay Rules

### GET `/api/v1/rules`

查询参数：

- `keyword`
- `status`
- `entryGroupId`
- `exitGroupId`
- `protocol`
- `page`
- `pageSize`

### POST `/api/v1/rules`

请求字段草案：

```json
{
  "name": "HK game relay",
  "entry": {
    "groupId": "group_hk_entry",
    "listenHost": "",
    "listenPort": "20000-20100",
    "protocol": "tcp"
  },
  "targets": [
    {
      "groupId": "group_us_exit",
      "host": "example.com",
      "port": 443,
      "weight": 100
    }
  ],
  "strategy": "fallback",
  "proxyProtocol": {
    "enabled": true,
    "version": "v2",
    "mode": "send"
  },
  "enabled": true
}
```

### GET `/api/v1/rules/:id`

### PATCH `/api/v1/rules/:id`

### DELETE `/api/v1/rules/:id`

### POST `/api/v1/rules/:id/start`

### POST `/api/v1/rules/:id/pause`

### POST `/api/v1/rules/batch-import`

### GET `/api/v1/rules/batch-export`

### POST `/api/v1/rules/:id/sync`

手动触发同步，实际实现可先 mock。

## Users

### GET `/api/v1/users`

### POST `/api/v1/users`

### GET `/api/v1/users/:id`

### PATCH `/api/v1/users/:id`

### DELETE `/api/v1/users/:id`

### POST `/api/v1/users/:id/disable`

### POST `/api/v1/users/:id/enable`

## Online IPs

### GET `/api/v1/online-ips`

查询参数：

- `keyword`
- `ruleId`
- `entryNodeId`
- `timeRange`
- `page`
- `pageSize`

### POST `/api/v1/online-ips`

### GET `/api/v1/online-ips/:id`

### PATCH `/api/v1/online-ips/:id`

### DELETE `/api/v1/online-ips/:id`

### POST `/api/v1/online-ips/:id/close`

## Health Checks

### GET `/api/v1/health-checks`

查询参数：

- `keyword`
- `status`：`healthy`、`warning`、`failed`、`unknown`、`disabled`
- `targetType`：`device`、`rule`、`service`
- `page`
- `pageSize`

### POST `/api/v1/health-checks`

请求字段草案：

```json
{
  "name": "香港入口 TCP 探针",
  "targetType": "device",
  "targetId": 1,
  "targetName": "hk-entry-01",
  "protocol": "tcp",
  "host": "hk.example.com",
  "port": 20000,
  "path": "",
  "intervalSec": 60,
  "timeoutSec": 5,
  "remark": "入口连通性"
}
```

### GET `/api/v1/health-checks/:id`

### PATCH `/api/v1/health-checks/:id`

### DELETE `/api/v1/health-checks/:id`

### POST `/api/v1/health-checks/:id/run`

手动执行一次真实探针。当前支持 TCP 握手、HTTP/HTTPS GET 探测；后台调度器会按 `intervalSec` 自动执行启用中的探针。

### GET `/api/v1/health-checks/:id/results`

返回该探针历史结果，包含状态、延迟、失败原因和检查时间。

## Certificates

### GET `/api/v1/certificates`

### POST `/api/v1/certificates`

### GET `/api/v1/certificates/:id`

### PATCH `/api/v1/certificates/:id`

### DELETE `/api/v1/certificates/:id`

### POST `/api/v1/certificates/:id/renew`

## Logs

### GET `/api/v1/logs/events`

### GET `/api/v1/logs/audit`

查询参数：

- `keyword`
- `type`
- `level`
- `from`
- `to`
- `page`
- `pageSize`
