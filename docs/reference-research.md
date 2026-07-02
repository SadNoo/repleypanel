# 参考面板调研记录

## 参考站

- 地址：参考演示站地址不写入公开仓库。
- 账号：`admin`
- 说明：
  - 数据每天 0 点（UTC+8）清空。
  - 部分功能有限制：规则数、用户数、设备数等。
  - 已禁用远程命令执行功能。
  - 已禁止设备接入，对接后不会上线。

## 当前可确认技术信息

- 站点前置了 Cloudflare。
- 直接 HTTP 抓取返回 Cloudflare challenge：
  - `HTTP/2 403`
  - `server: cloudflare`
  - `cf-mitigated: challenge`
  - CSP 允许 Cloudflare challenge 相关资源。
- 初次浏览器访问会停在 Cloudflare 安全验证页。
- 浏览器验证通过后可进入后台登录页。
- 登录后首页信息：
  - 产品：`nyanpass`
  - 面板版本：`20260618`
  - 页面标题：`demo`
  - 官方文档：参考项目文档站。
  - 站点公告：演示站已禁止设备接入，对接设备不会上线。

## 已确认前端栈

从页面静态资源与运行页面确认：

- React `18.2.0`
- React DOM `18.2.0`
- Ant Design `5.12.8`
- Ant Design Charts `2.0.3`
- Day.js `1.11.10`
- Lodash `4.17.21`
- Monaco Editor `0.45.0`
- Hash 路由：登录页 URL 为 `#/login`，登录后为 `#/`
- 构建产物：
  - `/assets/index-*.js`
  - `/assets/index-*.css`

补充判断：

- UI 组件库明显采用 Ant Design。
- 页面支持暗色主题。
- 首页在窄屏下为移动布局，左上角菜单折叠。
- Monaco Editor 说明后台存在高级 JSON/配置编辑类页面。

## 已确认路由模式

参考站使用 hash 路由，主路由与页面结构详见：

- `docs/reference-pages.md`

已观察到的 fetch/API 路径详见：

- `docs/reference-api-observed.md`

## 官方文档线索

文档站：参考项目文档站。

已确认栏目：

- 介绍与安装。
- 面板端：
  - 常用命令。
  - 完整配置。
  - 转发规则。
  - 设备组。
  - 探针。
  - 支付。
  - 个性定制。
- 节点端：
  - 常用命令。
  - 环境变量。
  - 常见疑难问题。
  - 资源限制。
  - 配置文件模式。
- 杂项：
  - 高频 DNS 缓存。
  - 连接信息与协议。
  - 基于 SNI 分流的转发。
  - CDN 隧道。
  - 专线对接。

业务模型线索：

- 转发规则支持批量导入/导出。
- 新格式批量规则为 JSON，包含转发规则详细设置。
- `listen_port = 0` 可用于随机端口。
- 转发规则由入口和出口分别同步，同步间隔约 20s。
- 单条规则创建/更新后面板会尝试通过 gRPC 立即推送给节点端，但仍可能等待同步。
- 规则状态包含“未同步”“正常”“同步失败”等。
- 入口设备组代表共用入口规则，不自带负载均衡。
- 出口设备组默认以最小连接数方式在组内均衡，并启用故障转移与健康检查。
- 流量扣除涉及入口倍率与出口倍率。
- 设备组 config 使用 JSON，高级编辑需要 Monaco 这类编辑器。
- 支持入口设置、出口设置、TLS 入站、TCP 隧道、UDP 转发、入口直出、反向隧道、链式出口、CDN 隧道等高级场景。

## 待继续进入后台后补充

- 登录页结构、表单字段、错误提示、会话存储方式。
- 前端框架与构建栈：
  - HTML/JS 资源命名。
  - 具体应用代码中的路由表。
  - 具体应用代码中的 API 封装。
  - 状态管理：Pinia、Vuex、Redux、Zustand 等。
- API 风格：
  - REST、GraphQL、RPC 或混合。
  - 鉴权方式：Cookie Session、JWT、Bearer Token、CSRF。
  - 分页、搜索、排序、过滤参数规范。
  - 错误码和响应 envelope。
- 页面模块：
  - 仪表盘。
  - 用户管理。
  - 设备管理。
  - 规则管理。
  - 设备组管理。
  - 隧道/中转管理。
  - 日志与审计。
  - 系统设置。
- 实时能力：
  - WebSocket、SSE、轮询或手动刷新。
- 数据清空逻辑：
  - 哪些数据每日清空。
  - 哪些配置长期保留。

## 本地旧面板参考

旧面板目录：`/Users/sadno/Downloads/xcode/relay-panel-web`

已实现为静态原型，包含：

- 概览：节点状态、活跃连接、流量、真实 IP 捕获率、实时拓扑和事件。
- 节点：在线状态、负载、延迟、流量。
- 中转规则：入口监听、出口目标、策略、Proxy Protocol、批量管理入口。
- 在线 IP：真实 IP 排行、连接明细。
- 证书：证书剩余时间、自动续期状态、使用范围。

旧面板建议 API：

- `GET /api/overview`
- `GET /api/nodes`
- `GET /api/rules`
- `GET /api/online-ips`
- `GET /api/certificates`
- `POST /api/rules`
