# 参考面板已观察接口

来源：参考演示站已登录管理员账号后的浏览器 `fetch` 资源清单。

说明：

- 以下是实际观察到的请求路径，主要是页面加载触发的读取接口。
- 未执行保存、删除、清空、充值、下单等写操作，因此 `POST`、`PATCH`、`DELETE` 路径暂不声称为已观察。
- 带分页参数的接口可抽象为支持 `page` 与 `size`。
- 外部配置服务用于客户端配置/更新信息，不属于本站业务 API。

## 外部配置/更新

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `<external-config-service>/download/nyanpass_config.json?t=...` | 面板远程配置 |
| GET | `<external-config-service>/download/nyanpass_update.json?t=...` | 面板更新信息 |

## Guest / System / User 基础接口

| 方法 | 路径 | 触发页面 | 用途 |
| --- | --- | --- | --- |
| GET | `/api/v1/guest/kv/site_info` | 首页/全局 | 站点信息 |
| GET | `/api/v1/system/info` | 首页/全局 | 系统信息、版本/授权等 |
| GET | `/api/v1/system/info/queue` | 管理仪表盘 | 队列情况 |
| GET | `/api/v1/system/node/status` | LookingGlass/节点状态 | 节点状态 |
| GET | `/api/v1/user/info` | 全局/个人中心 | 当前用户信息 |
| GET | `/api/v1/user/kv/site_notice` | 首页 | 用户侧站点公告 |

## 用户侧接口

| 方法 | 路径 | 触发页面 | 用途 |
| --- | --- | --- | --- |
| GET | `/api/v1/user/aff/config` | 个人中心 | 邀请/返佣配置 |
| GET | `/api/v1/user/devicegroup` | 单端隧道/转发规则 | 用户可用设备组 |
| GET | `/api/v1/user/forward/folder` | 转发规则 | 转发规则分组 |
| GET | `/api/v1/user/forward?page=1&size=10` | 转发规则 | 用户转发规则列表 |
| GET | `/api/v1/user/shop/payment_info` | 商城 | 用户侧支付信息 |
| GET | `/api/v1/user/shop/plan` | 商城 | 可购买套餐 |
| GET | `/api/v1/user/shop/order?page=1&size=10` | 我的订单 | 用户订单列表 |

## 管理侧接口

| 方法 | 路径 | 触发页面 | 用途 |
| --- | --- | --- | --- |
| GET | `/api/v1/admin/statistic?top_users=20` | 管理仪表盘 | 统计数据、排行榜 |
| GET | `/api/v1/admin/kv/site_notice` | 站点设置 | 管理侧站点公告 |
| GET | `/api/v1/admin/kv/payment_info` | 站点设置 | 支付配置 |
| GET | `/api/v1/admin/kv/invite_config` | 站点设置 | 邀请配置 |
| GET | `/api/v1/admin/kv/telegram-bot-config` | 推送通知 | Telegram Bot 配置 |
| GET | `/api/v1/admin/kv/device-offline-notify-config` | 推送通知 | 设备离线通知配置 |
| GET | `/api/v1/admin/devicegroup?` | 设备组管理/站点设置 | 管理员设备组列表 |
| GET | `/api/v1/admin/devicegroup/folder` | 管理员设备组 | 设备组分组 |
| GET | `/api/v1/admin/user?page=1&size=10` | 用户管理 | 用户列表 |
| GET | `/api/v1/admin/usergroup` | 用户组管理/用户管理 | 用户组列表 |
| GET | `/api/v1/admin/shop/plan` | 套餐管理/站点设置 | 套餐列表 |
| GET | `/api/v1/admin/shop/order?page=1&size=10` | 订单管理 | 管理员订单列表 |
| GET | `/api/v1/admin/shop/redeem?page=1&size=10` | 兑换码管理 | 兑换码列表 |
| GET | `/api/v1/admin/aff/log?page=1&size=10` | 邀请记录 | 返佣/邀请记录 |

## 观察到的接口命名规律

- Base URL：`/api/v1`。
- 权限域：
  - `/guest/...`：无需登录或弱权限信息。
  - `/user/...`：当前用户侧功能。
  - `/admin/...`：管理员功能。
  - `/system/...`：系统状态/运行信息。
- KV 配置：
  - `/api/v1/user/kv/:key`
  - `/api/v1/admin/kv/:key`
- 列表分页：
  - 常见查询参数：`page=1&size=10`。
- 业务域：
  - `forward`：转发规则。
  - `devicegroup`：设备组。
  - `shop/plan`：套餐。
  - `shop/order`：订单。
  - `shop/redeem`：兑换码。
  - `aff`：邀请/返佣。

## RepleyPass API 设计建议

如果想贴近参考站，RepleyPass 可以采用兼容风格：

- `/api/v1/user/forward`
- `/api/v1/user/forward/folder`
- `/api/v1/user/devicegroup`
- `/api/v1/admin/devicegroup`
- `/api/v1/admin/devicegroup/folder`
- `/api/v1/admin/user`
- `/api/v1/admin/usergroup`
- `/api/v1/admin/shop/plan`
- `/api/v1/admin/shop/order`
- `/api/v1/admin/shop/redeem`
- `/api/v1/admin/kv/:key`

如果想更现代、资源化，也可以在内部用 RESTful 命名，再在网关层兼容 nyanpass 风格路径。
