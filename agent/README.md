# RepleyPass Agent

RepleyPass Agent 是节点侧客户端后端，用于连接控制面、拉取规则配置、维护心跳，并执行入口侧中转。

## 功能

- 连接控制面：`/api/v1/agent/register`
- 周期心跳：`/api/v1/agent/heartbeat`
- 周期拉取配置：`/api/v1/agent/config`
- 在线连接上报：`/api/v1/agent/connections`
- 真实 IP：
  - 直连入口可读取 TCP `RemoteAddr`
  - 支持 Proxy Protocol v1/v2 解析
  - reverse tunnel open 会携带 `sourceIp/sourcePort/realIpSource`
  - 出口节点可按配置向最终目标发送 Proxy Protocol v1/v2
- 已实现入口 handler：
  - TCP 转发
  - UDP 转发
  - TLS passthrough
  - HTTP CONNECT
  - SOCKS5
- 已实现控制面中继隧道：
  - WS/WSS 协议入口
  - reverse tunnel
  - stream 多路复用
  - 入口节点到出口节点跨节点转发

## 运行

```bash
go run . \
  -control http://control.example.com \
  -token <agent_token> \
  -advertise node.example.com:20000 \
  -region HK
```

也可以通过环境变量配置：

```bash
REPLEYPASS_CONTROL_URL=http://control.example.com
REPLEYPASS_AGENT_TOKEN=<agent_token>
REPLEYPASS_AGENT_ADDRESS=node.example.com:20000
REPLEYPASS_AGENT_REGION=HK
REPLEYPASS_AGENT_VERSION=edge-0.1.0
REPLEYPASS_AGENT_CONFIG_SEC=20
REPLEYPASS_AGENT_HEARTBEAT_SEC=30
REPLEYPASS_AGENT_REPORT_SEC=15
```

## 规则要求

TCP/UDP/TLS passthrough 规则需要有明确的 `targetHost` 和 `targetPort`，Agent 才能直接转发。

HTTP CONNECT 与 SOCKS5 会按客户端请求的目标地址转发，适合做通用代理入口。

WS/WSS 与 reverse tunnel 使用控制面的 `/api/v1/agent/tunnel` 升级连接作为 broker。入口节点监听本地端口后，将每条客户端连接封装为 stream，经控制面转发到出口节点；出口节点再拨最终 `targetHost:targetPort`。
