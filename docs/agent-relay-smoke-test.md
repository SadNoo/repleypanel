# Agent 双节点联调基础流程

本文档用于验证入口 Agent、出口 Agent、控制面 tunnel broker 和真实 IP 元数据链路。所有地址均为占位符，实际部署时使用环境变量或运维侧配置，不写入仓库。

## 准备

1. 在控制面创建两个设备：
   - 入口设备：加入入口设备组。
   - 出口设备：加入出口设备组。
2. 分别为两个设备生成 Agent Token。
3. 创建一条 reverse tunnel 规则：
   - 入口组：入口设备所在组。
   - 出口组：出口设备所在组。
   - 协议：`reverse`、`ws` 或 `wss`。
   - 监听端口：入口节点本地监听端口。
   - 目标地址：出口节点可访问的最终服务域名与端口。
   - Proxy Protocol：按测试需要选择关闭、接收、v1 发送或 v2 发送。

## 启动 Agent

入口节点：

```bash
REPLEYPASS_CONTROL_URL=https://control.example.com \
REPLEYPASS_AGENT_TOKEN=<entry-agent-token> \
REPLEYPASS_AGENT_ADDRESS=entry-node.example.com:<listen-port> \
REPLEYPASS_AGENT_REGION=entry \
./repleypass-agent
```

出口节点：

```bash
REPLEYPASS_CONTROL_URL=https://control.example.com \
REPLEYPASS_AGENT_TOKEN=<exit-agent-token> \
REPLEYPASS_AGENT_ADDRESS=exit-node.example.com \
REPLEYPASS_AGENT_REGION=exit \
./repleypass-agent
```

## 验证项

- 两个设备在面板中显示为在线。
- 入口 Agent 日志出现规则监听记录。
- 出口 Agent 日志出现 tunnel connected。
- 入口 Agent 拉到的规则 `mode` 为 `reverse_tunnel`，`entry.enabled` 为 `true`，`tunnel.peerDeviceIds` 包含出口设备。
- 出口 Agent 拉到的规则 `mode` 为 `exit_only` 或角色为 `exit`，不会启动入口监听。
- 客户端连接入口端口后，流量能到达最终目标服务。
- 在线 IP 页面能看到连接记录。
- reverse tunnel open 元数据包含：
  - `ruleId`
  - `sourceIp`
  - `sourcePort`
  - `realIpSource`
  - `targetAddr`
- 当规则暂停、设备禁用、token 过期或目标设备不在出口组时，tunnel open 会被控制面拒绝。

## Proxy Protocol 测试

接收真实来源：

1. 在入口前面放置支持 Proxy Protocol 的四层转发器。
2. 规则设置为接收 Proxy Protocol。
3. 客户端连接后，在线 IP 应显示 Proxy Protocol 中的来源地址。

发送到最终目标：

1. 最终目标服务开启 Proxy Protocol v1 或 v2 接收。
2. 规则设置为对应版本的发送模式。
3. 出口 Agent 连接最终目标时会先写入 Proxy Protocol header。

## 故障判断

- `invalid agent token`：token 不存在、已轮换、已过期，或设备已禁用。
- `source device is not in rule entry group`：入口设备组与规则不匹配。
- `target device is not in rule exit group`：出口设备组与规则不匹配。
- `targetAddr does not match rule target`：Agent 上报的目标与规则目标不一致。
- `target device tunnel offline`：出口 Agent 未连接 tunnel。
