# RepleyPass Go API

Go 后端用于承接复刻前端，兼容参考站已观察到的 `/api/v1` 路径。

当前版本已经接入 SQLite 持久化、登录 session，并实现转发规则、设备组、设备/节点、在线 IP、健康检查 CRUD，以及节点 Agent 控制面接口。

转发规则已经扩展为业务化结构：入口组、监听地址/端口、出口组、目标端口、策略、Proxy Protocol、同步状态、运行统计、批量导入导出和审计日志。

在线 IP 用于记录运行态连接会话，并回写规则当前连接数、最近命中和入口设备连接数。

健康检查用于记录设备、规则或服务探针配置，支持真实 TCP/HTTP/HTTPS 探测、后台定时调度，保存每次检查结果、延迟和错误信息，并把失败/告警状态汇总到仪表盘。

节点 Agent 使用设备 token 鉴权，后台通过 `POST /api/v1/devices/{id}/agent-token` 生成 token，数据库仅保存哈希、过期时间和轮换时间。Agent 端使用 `Authorization: Bearer <token>` 调用注册、心跳、配置拉取、连接上报和 tunnel 接口。

## 本地运行

```bash
cd backend
go run .
```

默认监听：

```text
:8080
```

可通过环境变量覆盖监听地址和数据库路径：

```bash
REPLEYPASS_ADDR=:8081 REPLEYPASS_DB=./repleypass.db go run .
```

健康检查调度器默认每 30 秒扫描一次到期探针，可通过环境变量调整：

```bash
REPLEYPASS_HEALTH_SCHEDULER_SEC=10 go run .
```

首次启动会自动建表并写入演示数据。创建管理员账号时必须设置 `REPLEYPASS_ADMIN_PASSWORD`，项目不会提供默认管理员密码。

## 存储

- 默认 SQLite：`./repleypass.db`
- 线上 SQLite：`/var/lib/repleypass/repleypass.db`
- 表：`users`、`sessions`、`forward_rules`、`device_groups`、`devices`、`online_ips`、`health_checks`、`health_check_results`、`user_groups`、`plans`、`orders`、`redeem_codes`、`kv_settings`、`audit_logs`
- `devices` 已包含 Agent 字段：`agent_token_hash`、`agent_registered_at`、`agent_token_expires_at`、`agent_token_rotated_at`、`config_version`

## 已实现接口

- `GET /healthz`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/logout`
- `GET /api/v1/auth/me`
- `GET /api/v1/system/info`
- `GET /api/v1/system/info/queue`
- `GET /api/v1/system/node/status`
- `GET /api/v1/dashboard/overview`
- `GET /api/v1/dashboard/topology`
- `GET /api/v1/devices`
- `POST /api/v1/devices`
- `GET /api/v1/devices/{id}`
- `PATCH /api/v1/devices/{id}`
- `DELETE /api/v1/devices/{id}`
- `POST /api/v1/devices/{id}/enable`
- `POST /api/v1/devices/{id}/disable`
- `POST /api/v1/devices/{id}/heartbeat`
- `POST /api/v1/devices/{id}/agent-token`
- `POST /api/v1/agent/register`
- `POST /api/v1/agent/heartbeat`
- `GET /api/v1/agent/config`
- `POST /api/v1/agent/connections`
- `GET /api/v1/online-ips`
- `POST /api/v1/online-ips`
- `GET /api/v1/online-ips/{id}`
- `PATCH /api/v1/online-ips/{id}`
- `DELETE /api/v1/online-ips/{id}`
- `POST /api/v1/online-ips/{id}/close`
- `GET /api/v1/health-checks`
- `POST /api/v1/health-checks`
- `GET /api/v1/health-checks/{id}`
- `PATCH /api/v1/health-checks/{id}`
- `DELETE /api/v1/health-checks/{id}`
- `POST /api/v1/health-checks/{id}/run`
- `GET /api/v1/health-checks/{id}/results`
- `GET /api/v1/user/info`
- `GET /api/v1/user/kv/site_notice`
- `GET /api/v1/user/aff/config`
- `GET /api/v1/user/forward`
- `POST /api/v1/user/forward`
- `GET /api/v1/user/forward/{id}`
- `PATCH /api/v1/user/forward/{id}`
- `DELETE /api/v1/user/forward/{id}`
- `POST /api/v1/rules/{id}/start`
- `POST /api/v1/rules/{id}/pause`
- `POST /api/v1/rules/{id}/sync`
- `POST /api/v1/rules/batch-import`
- `GET /api/v1/rules/batch-export`
- `GET /api/v1/user/forward/folder`
- `GET /api/v1/user/devicegroup`
- `GET /api/v1/user/shop/payment_info`
- `GET /api/v1/user/shop/plan`
- `GET /api/v1/user/shop/order`
- `GET /api/v1/admin/statistic`
- `GET /api/v1/admin/kv/site_notice`
- `GET /api/v1/admin/kv/payment_info`
- `GET /api/v1/admin/kv/invite_config`
- `GET /api/v1/admin/kv/telegram-bot-config`
- `GET /api/v1/admin/kv/device-offline-notify-config`
- `GET /api/v1/admin/user`
- `GET /api/v1/admin/devicegroup`
- `POST /api/v1/admin/devicegroup`
- `GET /api/v1/admin/devicegroup/{id}`
- `PATCH /api/v1/admin/devicegroup/{id}`
- `DELETE /api/v1/admin/devicegroup/{id}`
- `GET /api/v1/admin/devicegroup/folder`
- `GET /api/v1/admin/usergroup`
- `GET /api/v1/admin/shop/plan`
- `GET /api/v1/admin/shop/order`
- `GET /api/v1/admin/shop/redeem`
- `GET /api/v1/admin/aff/log`
- `GET /api/v1/logs/audit`

更多兼容路径见 `main.go`。

## Agent 安全约束

- token 默认有效期为 30 天，可在签发时传入 `ttlHours` 或 `expiresAt`。
- 每次重新签发 token 都会覆盖旧 token 哈希，旧 token 立即失效。
- 禁用设备无法注册、心跳、拉取配置、上报连接或接入 tunnel。
- `/api/v1/agent/config` 只返回当前设备所在入口/出口组相关的规则。
- tunnel open 必须携带 `ruleId` 与 `targetAddr` 元数据；控制面会校验来源设备属于入口组、目标设备属于出口组、规则启用且目标地址匹配规则后才允许转发。

资源风格别名：

- `/api/v1/rules`
- `/api/v1/rules/{id}`
- `/api/v1/device-groups`
- `/api/v1/device-groups/{id}`
- `/api/v1/admin/device`
- `/api/v1/admin/device/{id}`
- `/api/v1/connections`
- `/api/v1/connections/{id}`

## 线上部署

当前服务器部署方式：

- API 监听：`localhost:8080`
- 数据库：`/var/lib/repleypass/repleypass.db`
- systemd：`repleypass-api.service`
- Nginx：`/api/` 反代到 Go API，其他路径走静态前端。

常用命令：

```bash
systemctl status repleypass-api.service
systemctl restart repleypass-api.service
systemctl status nginx
```
