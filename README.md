# RepleyPass

RepleyPass 是一个中转管理面板项目，包含静态前端、Go API 后端和节点 Agent。当前版本以参考面板的页面结构和接口路径为基础，实现了规则、设备、设备组、在线连接、健康检查、节点接入和控制面隧道等核心能力。

线上部署地址由运维环境配置，不在仓库中记录。

## 组成

- `index.html`、`assets/`：前端页面，使用静态 HTML/CSS/JS 实现。
- `backend/`：Go API 后端，使用 SQLite 持久化，提供 `/api/v1` 接口。
- `agent/`：节点 Agent 客户端后端，负责注册、心跳、拉取配置、上报连接和执行转发。
- `docs/`：需求整理、接口草案和参考面板调研文档。

## 当前功能

- 登录 session、用户信息、仪表盘概览。
- 转发规则 CRUD、启动/暂停、批量导入导出、审计记录。
- 设备组和设备/节点 CRUD，在线/离线/禁用状态，心跳和指标字段。
- 在线 IP、连接会话、健康检查和历史结果。
- Agent 控制面：注册、心跳、配置同步、连接上报、HTTP Upgrade tunnel。
- Agent 转发：TCP、UDP、TLS passthrough、HTTP CONNECT、SOCKS5。
- 真实 IP：直连入口读取 `RemoteAddr`，支持 Proxy Protocol v1/v2 解析；reverse tunnel open 元数据携带 `sourceIp`、`sourcePort`、`realIpSource`，出口可按规则向最终目标发送 Proxy Protocol v1/v2。

## 本地构建

构建 API 后端：

```bash
cd backend
go mod tidy
go build -o repleypass-api .
```

构建 Agent：

```bash
cd agent
go mod tidy
go build -o repleypass-agent .
```

## 部署

API 后端常用环境变量：

```bash
REPLEYPASS_ADDR=localhost:8080
REPLEYPASS_DB=/var/lib/repleypass/repleypass.db
REPLEYPASS_ADMIN_PASSWORD=<admin-password>
```

首次启动创建管理员账号时必须设置 `REPLEYPASS_ADMIN_PASSWORD`，项目不会提供默认管理员密码。

启动 API：

```bash
./backend/repleypass-api
```

静态前端可以部署到 Nginx：

```nginx
server {
    listen 80;
    server_name _;

    root /var/www/repleypass;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

Agent 运行示例：

```bash
./agent/repleypass-agent \
  -control http://control.example.com \
  -token <agent-token> \
  -advertise node.example.com:20000 \
  -region HK
```

Agent 也支持环境变量：

```bash
REPLEYPASS_CONTROL_URL=http://control.example.com
REPLEYPASS_AGENT_TOKEN=<agent-token>
REPLEYPASS_AGENT_ADDRESS=node.example.com:20000
REPLEYPASS_AGENT_REGION=HK
```

## 常用检查

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/api/v1/devices
curl http://localhost:8080/api/v1/system/node/status
```

## 文档

- [需求整理](docs/requirements.md)
- [API 草案](docs/api-draft.md)
- [参考面板页面结构](docs/reference-pages.md)
- [参考面板已观察接口](docs/reference-api-observed.md)
- [Go API 后端](backend/README.md)
- [节点 Agent](agent/README.md)
- [Agent 双节点联调基础流程](docs/agent-relay-smoke-test.md)
