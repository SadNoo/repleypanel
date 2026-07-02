(function () {
  const state = {
    authed: localStorage.getItem("repleypass-auth") !== "0",
    route: normalizeRoute(location.hash),
    collapsed: false,
    sidebarOpen: false,
    modal: null,
    modalKind: "",
    apiLoaded: false,
    apiError: "",
    user: null,
    system: null,
    overview: null,
  };

  const menu = [
    ["home", "主页", ""],
    ["user", "个人中心", "userinfo"],
    ["list", "转发规则", "forward_rules"],
    ["desktop", "单端隧道", "device_group"],
    ["shop", "商城", "shop"],
    ["order", "我的订单", "orders"],
    ["aim", "在线 IP", "online_ips"],
    ["aim", "LookingGlass", "looking_glass"],
    ["api", "节点状态 (旧)", "tz"],
    ["api", "节点状态 (新)", "tz2"],
  ];

  const adminMenu = [
    ["dash", "仪表盘", "admin/main"],
    ["setting", "站点设置", "admin/settings"],
    ["bell", "推送通知", "admin/push_settings"],
    ["user", "用户管理", "admin/users"],
    ["cloud", "设备管理", "admin/devices"],
    ["aim", "健康检查", "admin/health_checks"],
    ["team", "用户组管理", "admin/user_group"],
    ["order", "订单管理", "admin/orders"],
    ["shop", "套餐管理", "admin/plans"],
    ["tag", "兑换码管理", "admin/redeem"],
    ["cloud", "设备组管理", "admin/device_group"],
    ["gift", "邀请记录", "admin/afflog"],
  ];

  const icons = {
    home: "⌂", user: "♙", list: "☷", desktop: "▣", shop: "⌑", order: "⇄",
    aim: "◎", api: "⛓", dash: "◴", setting: "⚙", bell: "◉", team: "♟",
    tag: "◇", cloud: "☁", gift: "▱", menu: "☰", sun: "☀", close: "×"
  };

  const rows = {
    rules: [
      ["HK 低延迟入口", "香港入口 A : 20000", "东京出口组 : 443", "fallback", "0", "0.00 GiB", "0", "-", badge("正常", "ok"), actions(["同步", "暂停", "删除"])],
      ["WSS 回源", "新加坡入口 : 443", "美国 LA : 8443", "ip_hash", "0", "0.00 GiB", "0", "-", badge("未同步", "warn"), actions(["同步", "启动", "删除"])],
      ["备用 TCP", "日本入口 : 18080", "德国出口组 : 18080", "least_conn", "0", "0.00 GiB", "0", "-", badge("已暂停", "off"), actions(["启动", "删除"])],
    ],
    userDeviceGroups: [
      ["1", "私人单端 #1", "出口", "0.00 GiB", "0", "演示站禁止设备接入", actions(["编辑", "设备", "删除"])],
    ],
    userOrders: [
      ["NP202607010001", "2026/7/1 10:18:20", "-", "钱包充值", "66 元", "充值", badge("待支付", "warn"), actions(["详情"])],
    ],
    lookingGlass: [
      ["HK Entry", "入口", badge("0 台在线", "off")],
      ["US-LA Exit", "出口", badge("0 台在线", "off")],
    ],
    nodeStatus: [
      ["hk-entry-01", "香港入口 A", "入口", badge("离线", "off"), "0", "0ms", "0%", "-", actions(["详情"])],
      ["tokyo-exit-01", "东京出口组", "出口", badge("离线", "off"), "0", "0ms", "0%", "-", actions(["详情"])],
    ],
    onlineIps: [
      ["client-hk.example.com", "HK 低延迟入口", "hk-entry-01", "TCP", badge("Proxy Protocol v2", "ok"), "2", "HK", "-", actions(["关闭"])],
      ["client-sg.example.com", "WSS 回源", "hk-entry-01", "WSS", badge("连接日志", "warn"), "1", "SG", "-", actions(["关闭"])],
    ],
    devices: [
      ["1", "hk-entry-01", "香港入口 A", "入口", badge("离线", "off"), "hk.example.com:20000", "HK", "0", "0ms", "0%", "-", actions(["编辑", "禁用", "删除"])],
      ["2", "tokyo-exit-01", "东京出口组", "出口", badge("离线", "off"), "tokyo-exit.example.com:443", "JP", "0", "0ms", "0%", "-", actions(["编辑", "禁用", "删除"])],
    ],
    healthChecks: [
      ["香港入口 TCP 探针", "设备 hk-entry-01", "TCP", "hk.example.com:20000", "60s", "5s", badge("健康", "ok"), "18ms", "-", "", actions(["执行", "结果", "删除"])],
    ],
    users: [
      ["1", "admin", "9999/9/9 上午8:09:09", "0.00 GiB / 55.00 GiB", "#55", "#1", "10", "66 元", actions(["编辑", "规则", "余额"]), ""],
    ],
    userRules: [
      ["admin", "HK 低延迟入口", "香港入口 A", "东京出口组", "0.00 GiB", badge("正常", "ok"), actions(["暂停", "删除"])],
    ],
    userGroups: [
      ["1", "#1", "默认管理组", "1", actions(["编辑", "删除"])],
    ],
    adminOrders: [
      ["NP202607010001", "admin", "2026/7/1 10:18:20", "-", "钱包充值", "66 元", "充值", badge("待支付", "warn"), actions(["记账", "删除"])],
    ],
    plans: [
      ["1", "演示套餐", "周期", "#1", "55.00 GiB", "10", "0 元", badge("否", "ok"), actions(["编辑", "隐藏", "删除"])],
    ],
    redeem: [
      ["1", "DEMO-2026", "演示套餐", "100%", "10", actions(["复制", "删除"])],
    ],
    adminDeviceGroups: [
      ["1", "香港入口 A", "#1", "入口", "hk.example.com", "1.5x", "0.00 GiB", "0", "禁止接入", actions(["编辑", "设备", "高级"])],
      ["2", "东京出口组", "#1", "出口", "-", "0.5x", "0.00 GiB", "0", "最小连接数", actions(["编辑", "设备", "高级"])],
    ],
    afflog: [
      ["admin", "2026/7/1 10:18:20", "手动记账", "33 元", "返佣", badge("已记录", "ok"), actions(["详情", "删除"])],
    ],
  };

  window.addEventListener("hashchange", () => {
    state.route = normalizeRoute(location.hash);
    render();
  });

  document.addEventListener("click", async (event) => {
    const target = event.target.closest("[data-action], [data-route], [data-modal]");
    if (!target) return;
    const action = target.dataset.action;
    if (target.dataset.route !== undefined) {
      go(target.dataset.route);
      return;
    }
    if (target.dataset.modal) {
      state.modalKind = target.dataset.modal;
      state.modal = modalContent(target.dataset.modal);
      render();
      return;
    }
    if (action === "login") {
      await login();
      return;
    }
    if (action === "logout") {
      await logout();
      state.authed = false;
      localStorage.setItem("repleypass-auth", "0");
      location.hash = "#/login";
      render();
      return;
    }
    if (action === "toggle-sidebar") {
      state.sidebarOpen = !state.sidebarOpen;
      state.collapsed = !state.collapsed && window.innerWidth > 980;
      render();
      return;
    }
    if (action === "close-modal") {
      state.modal = null;
      state.modalKind = "";
      render();
      return;
    }
    if (action === "refresh") {
      await loadApiData();
      toast("刷新成功");
      return;
    }
    if (action === "export-rules") {
      await exportRules();
      return;
    }
    if (action === "run-health-check") {
      await runHealthCheck(target.dataset.id);
      return;
    }
    if (action === "show-health-results") {
      await showHealthResults(target.dataset.id, target.dataset.name);
      return;
    }
    if (action === "generate-agent-token") {
      await generateAgentToken(target.dataset.id, target.dataset.name);
      return;
    }
    if (action === "save") {
      await saveModal();
      return;
    }
    if (action === "noop") toast("演示站功能已禁用");
  });

  render();
  loadApiData();

  function render() {
    if (state.route === "login" || !state.authed) {
      document.getElementById("app").innerHTML = loginPage();
      return;
    }

    document.getElementById("app").innerHTML = `
      <div class="layout">
        ${topbar()}
        ${sidebar()}
        <main class="main">${pageFor(state.route)}</main>
        ${modal()}
        <div id="toast" class="toast"></div>
      </div>
    `;
  }

  function topbar() {
    return `
      <header class="topbar">
        <button class="icon text" data-action="toggle-sidebar" title="菜单">${icons.menu}</button>
        <h1 class="brand-title">demo</h1>
        <div class="top-spacer"></div>
        <div class="user-split">
          <button>${escapeHtml((state.user && state.user.username) || "admin")}</button>
          <button class="icon" data-action="logout" title="退出登录">${icons.user}</button>
        </div>
        <button class="icon text" title="主题">${icons.sun}</button>
      </header>
    `;
  }

  function sidebar() {
    const cls = `sidebar ${state.collapsed ? "collapsed" : ""} ${state.sidebarOpen ? "open" : ""}`;
    return `
      <aside class="${cls}">
        <nav class="menu-section">
          ${menu.map(item => menuItem(item)).join("")}
          <div class="menu-group">管理</div>
          <div class="submenu">${adminMenu.map(item => menuItem(item)).join("")}</div>
        </nav>
      </aside>
    `;
  }

  function menuItem([icon, label, route]) {
    const active = state.route === route || (route === "" && state.route === "");
    return `<div class="menu-item ${active ? "active" : ""}" data-route="${route}">
      <span class="menu-icon">${icons[icon]}</span><span>${label}</span>
    </div>`;
  }

  function loginPage() {
    return `
      <section class="login-page">
        <div class="login-card">
          <h1>登录</h1>
          <label class="login-field">
            <input id="loginUsername" value="admin" placeholder="用户名">
          </label>
          <label class="login-field">
            <input id="loginPassword" value="" type="password" placeholder="密码">
          </label>
          <button class="primary" data-action="login">登 录</button>
          <div class="login-extra">前往注册</div>
        </div>
      </section>
    `;
  }

  function pageFor(route) {
    const pages = {
      "": homePage,
      userinfo: userInfoPage,
      forward_rules: forwardRulesPage,
      device_group: userDeviceGroupPage,
      shop: shopPage,
      orders: userOrdersPage,
      online_ips: onlineIPsPage,
      looking_glass: lookingGlassPage,
      tz: nodeStatusPage,
      tz2: nodeStatusPage,
      "admin/main": adminDashboardPage,
      "admin/settings": settingsPage,
      "admin/push_settings": pushSettingsPage,
      "admin/users": adminUsersPage,
      "admin/devices": adminDevicesPage,
      "admin/health_checks": healthChecksPage,
      "admin/user_group": userGroupPage,
      "admin/orders": adminOrdersPage,
      "admin/plans": plansPage,
      "admin/redeem": redeemPage,
      "admin/device_group": adminDeviceGroupPage,
      "admin/afflog": afflogPage,
    };
    return (pages[route] || notFoundPage)();
  }

  function homePage() {
    const system = state.system || {};
    return page(`<h2>欢迎使用</h2>`, `
      <div class="card">
        <div class="card-body">
          <h2>欢迎使用</h2>
          <h2>${escapeHtml(system.panel || "nyanpass")} 面板版本: ${escapeHtml(system.version || "20260618")}</h2>
          <h2>面板授权到期时间: ${escapeHtml(system.licenseExpire || "2286/11/21 上午1:46:39")}</h2>
          <p><a href="#/docs" target="_blank">项目文档</a></p>
          <div class="notice">
            <div class="notice-title">站点公告</div>
            <div class="notice-body">这是演示站，已经禁止设备接入，您对接的设备不会上线。</div>
          </div>
          <div class="grid" style="margin-top:16px">
            <div class="collapse-row" data-action="noop"><span>›</span><strong>站点信息</strong></div>
            <div class="collapse-row" data-action="noop"><span>›</span><strong>后端信息</strong></div>
          </div>
        </div>
      </div>
    `);
  }

  function userInfoPage() {
    const u = state.user || {};
    return page(title("用户信息"), `
      <div class="grid cols-2">
        ${card("用户信息", `
          <div class="info-grid">
            ${info("用户名", escapeHtml(u.username || "admin"))}
            ${info("用户类型", escapeHtml(u.userType || "管理员"))}
            ${info("用户组", escapeHtml(u.userGroup || "#1"))}
            ${info("套餐", escapeHtml(u.plan || "#55"))}
            ${info("套餐失效", escapeHtml(u.planExpire || "9999/9/9 上午8:09:09"))}
            ${info("续费价格", "0 元")}
            ${info("流量", `${escapeHtml(u.trafficUsed || "0.00 GiB")} / ${escapeHtml(u.trafficTotal || "55.00 GiB")}`)}
            ${info("最大规则数", escapeHtml(u.maxRules || "10"))}
            ${info("速率限制", escapeHtml(u.rateLimit || "500 Mbps"))}
            ${info("连接数限制", escapeHtml(u.connectionLimit || "300"))}
            ${info("钱包余额", escapeHtml(u.walletBalance || "66 元"))}
            ${info("Telegram 关联", u.telegramLinked ? "1" : "0")}
          </div>
          <div class="toolbar" style="margin-top:18px">
            <button class="primary">立即续费</button><button>充值</button><button>关联</button><button>取消关联</button><button>设置</button>
          </div>
        `)}
        ${card("账户设置", `
          <div class="switch-line"><div><strong>自动续费</strong><p class="muted">余额充足时自动续费套餐或流量。</p></div><span class="switch"></span></div>
          <hr style="border-color:var(--line);border-style:solid;border-width:1px 0 0;margin:18px 0">
          <div class="form-grid">
            ${field("当前密码", "<input placeholder='当前密码' type='password'>")}
            ${field("新密码", "<input placeholder='新密码，留空随机生成。' type='password'>")}
            ${field("确认新密码", "<input placeholder='确认新密码' type='password'>")}
          </div>
          <div class="toolbar right" style="margin-top:18px"><button class="primary">重置密码</button></div>
        `)}
      </div>
      ${card("邀请注册", `
        <div class="info-grid">
          ${info("邀请注册链接", "您还没有邀请码，请先创建邀请码。")}
          ${info("佣金比例", "0.00%（首次消费返利）")}
          ${info("佣金余额", "33 元")}
        </div>
        <div class="toolbar" style="margin-top:18px"><button>查看邀请记录</button><button>重置邀请码</button><button>划转余额</button></div>
      `)}
    `);
  }

  function forwardRulesPage() {
    return page(pageTitle("我的转发规则", buttons(["搜索规则", "刷新", "统计数据"], "refresh")), `
      <div class="grid cols-3">
        ${stat("流量", "0.00 GiB / 55.00 GiB", "入口流量单向统计")}
        ${stat("到期", "9999/9/9 上午8:09:09", "套餐仍然有效")}
        ${stat("规则数", "3 / 10", "演示规则")}
      </div>
      ${cardHead("我的转发规则", `
        <div class="toolbar">
          <button data-modal="rule">添加规则</button><button data-modal="batch">批量导入</button><button data-action="export-rules">批量导出</button>
          <button>批量切换</button><button>清空流量</button><button class="danger">删除选中</button>
        </div>
      `, `
        <div class="toolbar" style="padding:16px 24px"><button>管理分组</button><span class="tag">全部</span><span class="tag">未分组 (3)</span></div>
        ${table(["规则名", "入口", "出口", "策略", "连接数", "今日流量", "错误", "最近命中", "状态", "健康", "操作"], rows.rules)}
      `)}
    `);
  }

  function userDeviceGroupPage() {
    return page(pageTitle("设备组管理 (我的单端隧道)", `<button data-modal="deviceGroup">添加设备组</button><button data-action="noop">清空流量</button><button data-action="refresh">刷新</button>`), `
      ${tableCard(["排序", "名称", "类型", "已用流量", "在线设备", "备注", "操作"], rows.userDeviceGroups)}
    `);
  }

  function shopPage() {
    return page(title("商城"), `
      <div class="grid cols-2">
        ${card("我的钱包", `${infoBlock("钱包余额", "66 元")}<div class="toolbar"><button class="primary">钱包充值</button></div>`)}
        ${card("钱包充值", `${field("充值金额", "<input value='0' type='number'>")}<p class="muted">CNY，最小充值金额: 0 元</p><p class="gold">站点未配置任何支付方式！</p>`)}
      </div>
      ${card("购买套餐", `<div class="empty">暂无数据</div>`)}
      ${card("兑换套餐", `<p class="muted">如果您有兑换码，则可以免费或低价购买对应的套餐。</p><div class="toolbar"><input placeholder="兑换码"><button class="primary">兑 换</button></div>`)}
    `);
  }

  function userOrdersPage() {
    return page(title("我的订单"), tableCard(["订单号", "创建时间", "支付时间", "订单信息", "金额", "类型", "状态", "操作"], rows.userOrders));
  }

  function onlineIPsPage() {
    return page(pageTitle("在线 IP", `<button data-modal="onlineIp">记录连接</button><button data-action="refresh">刷新</button>`), `
      ${tableCard(["来源 IP", "规则", "入口设备", "协议", "真实 IP 来源", "连接数", "地区", "最近活跃", "操作"], rows.onlineIps)}
    `);
  }

  function lookingGlassPage() {
    return page(title("LookingGlass"), tableCard(["名称", "类型", "在线设备"], rows.lookingGlass));
  }

  function nodeStatusPage() {
    return page(title(state.route === "tz" ? "节点状态 (旧)" : "节点状态 (新)"), `
      ${tableCard(["设备", "设备组", "类型", "状态", "健康", "连接数", "延迟", "负载", "最后心跳", "操作"], rows.nodeStatus)}
    `);
  }

  function adminDashboardPage() {
    const overview = state.overview || {};
    return page(title("数据一览"), `
      <div class="grid cols-4">
        ${stat("今日充值", "0.00 CNY", "昨日 0.00 CNY")}
        ${stat("在线节点", escapeHtml(overview.onlineNodes || 0), "来自设备心跳")}
        ${stat("活跃连接", escapeHtml(overview.activeConnections || 0), "来自在线 IP")}
        ${stat("告警", escapeHtml(overview.alerts || 0), "设备告警 + 探针异常")}
      </div>
      <div class="grid cols-2">
        ${rankCard("今日用户流量排行", [["admin", "0.00 GiB", 4]])}
        ${rankCard("昨日用户流量排行", [["admin", "0.00 GiB", 2]])}
        ${rankCard("今日节点流量排行", [["香港入口 A", "0.00 GiB", 1]])}
        ${rankCard("昨日节点流量排行", [["东京出口组", "0.00 GiB", 1]])}
      </div>
    `);
  }

  function settingsPage() {
    return page(title("站点设置"), `
      ${card("基本", `
        <div class="form-grid">
          ${field("站点名称", "<input value='demo'>")}
          ${field("邀请注册策略", select(["无限制", "必须邀请码", "禁止注册"]))}
          ${field("注册验证码", select(["无", "hCaptcha", "reCAPTCHA"]))}
          ${field("诊断结果隐藏 IP", select(["不隐藏", "隐藏"]))}
          ${field("主题策略", select(["仅允许经典主题", "允许透明主题"]))}
          ${field("透明主题背景图 URL（横屏视角）", "<input>")}
          ${field("透明主题背景图 URL（竖屏视角）", "<input>")}
          ${field("允许注册", "<span class='switch'></span>")}
          ${field("允许用户自带出口（单端隧道）", "<span class='switch'></span>")}
          ${field("允许 Looking Glass", "<span class='switch'></span>")}
        </div>
        <div class="toolbar right" style="margin-top:18px"><button class="primary" data-action="save">保 存</button></div>
      `)}
      ${card("站点公告", `<textarea placeholder="以 < 开头则显示为 HTML">这是演示站，已经禁止设备接入，您对接的设备不会上线。</textarea><div class="toolbar right" style="margin-top:12px"><button class="primary">保 存</button></div>`)}
      ${cardHead("支付设置", `<div class="toolbar"><button data-modal="payment">添加支付通道</button><button data-modal="json">编辑支付设置 json</button></div>`, `
        <div class="card-body">${field("最小充值金额", "<input value='0'>")}<p class="muted">元</p></div>
        ${table(["排序", "类型", "名称", "是否启用", "操作"], [])}
      `)}
      ${card("邀请设置 (定制功能，若需使用请咨询作者)", `
        <div class="form-grid">
          ${field("启用邀请注册", "<span class='switch off'></span>")}
          ${field("循环返佣", "<span class='switch off'></span>")}
          ${field("强制绑定 Telegram", "<span class='switch off'></span>")}
          ${field("返佣比例 (%)", "<input value='0'>")}
        </div>
      `)}
    `);
  }

  function pushSettingsPage() {
    return page(title("推送通知"), `
      ${card("Telegram Bot 推送通道", `
        <div class="form-grid">
          ${field("启用 Telegram Bot", "<span class='switch off'></span>")}
          ${field("Bot Token", "<input placeholder='请输入 Bot 的 Token'>")}
          ${field("Webhook URL", "<input value='https://example.com/api/v1/telegram/webhook'>")}
        </div>
        <div class="toolbar right" style="margin-top:18px"><button>自动填充</button><button class="primary">保存通道配置</button></div>
      `)}
      ${card("设备离线通知", `<p class="muted">设备最后一次心跳超过宽限期后标记为离线；保留期后移除在线会话信息。</p>`)}
      ${card("全局默认设置", `<div class="form-grid">${field("设备离线宽限期", "<input value='60'>")} ${field("设备离线保留期", "<input value='300'>")}</div>`)}
      ${cardHead("设备组覆盖设置", `<button class="primary">保存离线通知配置</button>`, table(["设备组", "自定义设备离线宽限期", "自定义设备离线保留期"], []))}
    `);
  }

  function adminUsersPage() {
    return page(pageTitle("用户管理", buttons(["显示列", "添加用户", "搜索规则", "清理无效用户", "清理无效规则", "刷新"], "refresh")), `
      ${tableCard(["UID", "用户名", "过期时间", "流量", "套餐", "用户组", "最大规则数", "钱包余额", "操作", "备注"], rows.users)}
      ${cardHead("找到 0 条规则", `<button>返回所有用户</button><button>批量切换</button><button class="danger">删除选中用户规则</button>`, table(["用户", "规则名", "入口", "出口", "已用流量", "状态", "操作"], rows.userRules))}
    `);
  }

  function adminDevicesPage() {
    return page(pageTitle("设备管理", `<button data-modal="device">添加设备</button><button data-action="refresh">刷新</button>`), `
      ${tableCard(["ID", "名称", "设备组", "类型", "状态", "健康", "Agent", "配置", "地址", "区域", "连接数", "延迟", "负载", "最后心跳", "操作"], rows.devices)}
    `);
  }

  function healthChecksPage() {
    return page(pageTitle("健康检查", `<button data-modal="healthCheck">添加探针</button><button data-action="refresh">刷新</button>`), `
      <div class="grid cols-3">
        ${stat("探针数", rows.healthChecks.length, "设备/规则探测")}
        ${stat("异常", rows.healthChecks.filter(row => String(row[6]).includes("异常") || String(row[6]).includes("失败") || String(row[6]).includes("告警")).length, "失败或告警")}
        ${stat("执行方式", "手动 / API", "后续接入调度器")}
      </div>
      ${tableCard(["名称", "目标", "协议", "地址", "间隔", "超时", "状态", "延迟", "最近检查", "错误", "操作"], rows.healthChecks)}
    `);
  }

  function userGroupPage() {
    return page(pageTitle("用户组管理", buttons(["添加用户组", "查看说明"])), tableCard(["排序", "用户组 ID", "名称", "用户数量", "操作"], rows.userGroups));
  }

  function adminOrdersPage() {
    return page(pageTitle("管理订单", buttons(["手动记账", "删除选中"])), tableCard(["订单号", "用户", "创建时间", "支付时间", "订单信息", "金额", "类型", "状态", "操作"], rows.adminOrders));
  }

  function plansPage() {
    return page(pageTitle("套餐管理", buttons(["添加套餐"])), tableCard(["排序", "名称", "类型", "分配用户组", "可用流量", "规则数", "价格", "隐藏", "操作"], rows.plans));
  }

  function redeemPage() {
    return page(pageTitle("兑换码管理", buttons(["批量添加兑换码", "删除选中"])), tableCard(["序号", "代码", "套餐", "折扣比例", "剩余次数", "操作"], rows.redeem));
  }

  function adminDeviceGroupPage() {
    return page(pageTitle("设备组管理 (站点管理员)", `<button data-modal="deviceGroup">添加设备组</button><button data-action="noop">清空流量</button><button data-action="noop">分组</button><button data-action="refresh">刷新</button><button data-action="noop">管理分组</button>`), `
      <div class="toolbar" style="margin-bottom:12px"><span class="tag">全部</span><span class="tag">未分组 (2)</span></div>
      ${tableCard(["排序", "名称", "用户组 ID", "类型", "连接地址(仅入口展示用)", "倍率", "已用流量", "在线设备", "备注", "操作"], rows.adminDeviceGroups)}
    `);
  }

  function afflogPage() {
    return page(pageTitle("管理邀请记录", buttons(["手动记账", "删除选中"])), tableCard(["用户", "创建时间", "订单信息", "金额", "类型", "状态", "操作"], rows.afflog));
  }

  function notFoundPage() {
    return page(title("页面不存在"), card("404", `<div class="empty">未找到当前路由：#/${escapeHtml(state.route)}</div>`));
  }

  function modalContent(kind) {
    const map = {
      rule: ["添加规则", `
        <div class="form-grid">
          ${field("规则名", "<input id='ruleName' value='HK 低延迟入口'>")}
          ${field("入口设备组 ID", "<input id='ruleEntryGroupId' value='1' type='number'>")}
          ${field("入口设备组", "<input id='ruleEntryGroupName' value='香港入口 A'>")}
          ${field("监听地址", "<input id='ruleListenHost' value=''>")}
          ${field("监听端口", "<input id='ruleListenPort' value='20000'>")}
          ${field("出口设备组 ID", "<input id='ruleExitGroupId' value='2' type='number'>")}
          ${field("出口设备组", "<input id='ruleExitGroupName' value='东京出口组'>")}
          ${field("目标地址", "<input id='ruleTargetHost' value=''>")}
          ${field("目标端口", "<input id='ruleTargetPort' value='443'>")}
          ${field("Proxy Protocol", "<select id='ruleProxy'><option>关闭</option><option>v1 发送</option><option>v2 发送</option></select>")}
          ${field("Proxy 模式", "<select id='ruleProxyMode'><option value='send'>发送</option><option value='receive'>接收</option><option value='off'>关闭</option></select>")}
          ${field("协议", "<select id='ruleProtocol'><option>TCP</option><option>TLS 入站</option><option>WS</option><option>HTTP</option></select>")}
          ${field("策略", "<select id='ruleStrategy'><option value='fallback'>Fallback</option><option value='round_robin'>Round Robin</option><option value='ip_hash'>IP Hash</option><option value='least_conn'>Least Conn</option></select>")}
          ${field("分组", "<select id='ruleGroup'><option>未分组</option><option>游戏</option><option>Web</option></select>")}
          ${field("备注", "<input id='ruleRemark' value=''>")}
        </div>
      `],
      deviceGroup: ["添加设备组", `
        <div class="form-grid">
          ${field("排序", "<input id='groupSort' value='4' type='number'>")}
          ${field("名称", "<input id='groupName' value='新设备组'>")}
          ${field("用户组 ID", "<input id='groupUserGroupId' value='#1'>")}
          ${field("类型", "<select id='groupType'><option>入口</option><option>出口</option></select>")}
          ${field("连接地址", "<input id='groupDisplayAddress' value='-'>")}
          ${field("倍率", "<input id='groupMultiplier' value='1' type='number' step='0.1'>")}
          ${field("备注", "<input id='groupRemark' value='后台新增'>")}
        </div>
      `],
      device: ["添加设备", `
        <div class="form-grid">
          ${field("名称", "<input id='deviceName' value='new-edge-01'>")}
          ${field("设备组 ID", "<input id='deviceGroupId' value='1' type='number'>")}
          ${field("设备组", "<input id='deviceGroupName' value='香港入口 A'>")}
          ${field("类型", "<select id='deviceType'><option>入口</option><option>出口</option></select>")}
          ${field("状态", "<select id='deviceStatus'><option value='offline'>离线</option><option value='online'>在线</option><option value='disabled'>禁用</option><option value='alert'>告警</option></select>")}
          ${field("地址", "<input id='deviceAddress' value='node.example.com:20000'>")}
          ${field("区域", "<input id='deviceRegion' value='HK'>")}
          ${field("版本", "<input id='deviceVersion' value='edge-0.1.0'>")}
          ${field("备注", "<input id='deviceRemark' value='后台新增设备'>")}
        </div>
      `],
      onlineIp: ["记录在线连接", `
        <div class="form-grid">
          ${field("来源 IP", "<input id='onlineSourceIp' value='client.example.com'>")}
          ${field("来源端口", "<input id='onlineSourcePort' value='50000' type='number'>")}
          ${field("规则 ID", "<input id='onlineRuleId' value='1' type='number'>")}
          ${field("规则名", "<input id='onlineRuleName' value='HK 低延迟入口'>")}
          ${field("入口设备 ID", "<input id='onlineEntryDeviceId' value='1' type='number'>")}
          ${field("入口设备", "<input id='onlineEntryDeviceName' value='hk-entry-01'>")}
          ${field("入口设备组", "<input id='onlineEntryGroupName' value='香港入口 A'>")}
          ${field("协议", "<select id='onlineProtocol'><option>TCP</option><option>WSS</option><option>HTTP</option><option>UDP</option></select>")}
          ${field("真实 IP 来源", "<select id='onlineRealIpSource'><option value='proxy_protocol_v2'>Proxy Protocol v2</option><option value='proxy_protocol_v1'>Proxy Protocol v1</option><option value='connection_log'>连接日志</option><option value='fallback'>回退识别</option></select>")}
          ${field("连接数", "<input id='onlineConnectionCount' value='1' type='number'>")}
          ${field("地区", "<input id='onlineCountry' value='HK'>")}
        </div>
      `],
      healthCheck: ["添加健康检查", `
        <div class="form-grid">
          ${field("名称", "<input id='checkName' value='新 TCP 探针'>")}
          ${field("目标类型", "<select id='checkTargetType'><option value='device'>设备</option><option value='rule'>规则</option><option value='service'>服务</option></select>")}
          ${field("目标 ID", "<input id='checkTargetId' value='1' type='number'>")}
          ${field("目标名称", "<input id='checkTargetName' value='hk-entry-01'>")}
          ${field("协议", "<select id='checkProtocol'><option value='tcp'>TCP</option><option value='http'>HTTP</option><option value='https'>HTTPS</option></select>")}
          ${field("主机", "<input id='checkHost' value='hk.example.com'>")}
          ${field("端口", "<input id='checkPort' value='20000' type='number'>")}
          ${field("路径", "<input id='checkPath' value='/'>")}
          ${field("间隔秒", "<input id='checkIntervalSec' value='60' type='number'>")}
          ${field("超时秒", "<input id='checkTimeoutSec' value='5' type='number'>")}
          ${field("备注", "<input id='checkRemark' value='后台新增探针'>")}
        </div>
      `],
      batch: ["批量导入", `${field("规则 JSON", "<textarea id='batchRulesJson'>{\\n  \"rules\": [\\n    {\\n      \"name\": \"批量导入规则\",\\n      \"entryGroupName\": \"香港入口 A\",\\n      \"listenPort\": \"21000\",\\n      \"exitGroupName\": \"东京出口组\",\\n      \"targetPort\": \"443\",\\n      \"strategy\": \"fallback\",\\n      \"protocol\": \"TCP\"\\n    }\\n  ]\\n}</textarea>")}`],
      payment: ["添加支付通道", `<div class="form-grid">${field("类型", select(["alipay", "wechat", "stripe"]))}${field("名称", "<input value='默认通道'>")}${field("是否启用", "<span class='switch'></span>")}</div>`],
      json: ["编辑支付设置 json", `<textarea>{\n  \"channels\": []\n}</textarea>`],
    };
    return map[kind] || ["演示操作", "<p>这是演示弹窗。</p>"];
  }

  function modal() {
    if (!state.modal) return "";
    return `
      <div class="modal-mask open">
        <div class="modal">
          <div class="modal-head"><h3>${state.modal[0]}</h3><button class="icon text" data-action="close-modal">${icons.close}</button></div>
          <div class="modal-body">${state.modal[1]}</div>
          <div class="modal-actions"><button data-action="close-modal">取 消</button><button class="primary" data-action="save">确 定</button></div>
        </div>
      </div>
    `;
  }

  function page(heading, body) { return `<section class="page">${heading}${body}</section>`; }
  function title(text) { return `<div class="page-title"><h2>${text}</h2></div>`; }
  function pageTitle(text, tools) { return `<div class="page-title"><div><h2>${text}</h2></div><div class="toolbar right">${tools || ""}</div></div>`; }
  function card(titleText, body) { return `<section class="card"><div class="card-head"><h3>${titleText}</h3></div><div class="card-body">${body}</div></section>`; }
  function cardHead(titleText, tools, body) { return `<section class="card"><div class="card-head"><h3>${titleText}</h3><div class="toolbar">${tools || ""}</div></div><div class="card-body tight">${body}</div></section>`; }
  function stat(label, value, note) { return `<div class="stat-card"><span>${label}</span><strong>${value}</strong><small>${note || ""}</small></div>`; }
  function info(label, value) { return `<div class="info-item"><span>${label}</span><strong>${value}</strong></div>`; }
  function infoBlock(label, value) { return `<div class="stat-card"><span>${label}</span><strong>${value}</strong></div>`; }
  function field(label, control) { return `<label class="form-field"><span>${label}</span>${control}</label>`; }
  function select(items) { return `<select>${items.map(x => `<option>${x}</option>`).join("")}</select>`; }
  function badge(text, tone) { return `<span class="status ${tone || ""}">${text}</span>`; }
  function actions(items) { return `<div class="toolbar">${items.map(x => `<button class="small">${x}</button>`).join("")}</div>`; }
  function buttons(items, action) { return items.map(x => `<button data-action="${x === "刷新" ? "refresh" : action || "noop"}">${x}</button>`).join(""); }

  function table(headers, data) {
    return `<div class="table-wrap"><table><thead><tr>${headers.map(h => `<th>${h}</th>`).join("")}</tr></thead><tbody>${
      data.length ? data.map(row => `<tr>${row.map(cell => `<td>${cell}</td>`).join("")}</tr>`).join("") : `<tr><td colspan="${headers.length}" class="empty">暂无数据</td></tr>`
    }</tbody></table></div>`;
  }

  function tableCard(headers, data) {
    return `<section class="card"><div class="card-body tight">${table(headers, data)}</div></section>`;
  }

  function rankCard(titleText, data) {
    return card(titleText, data.map(([name, value, pct]) => `
      <div class="info-item"><span>${name}</span><strong>${value}</strong></div>
      <div class="progress" style="margin:8px 0 14px"><span style="width:${pct}%"></span></div>
    `).join(""));
  }

  function go(route) {
    state.sidebarOpen = false;
    location.hash = "#/" + route;
  }

  function normalizeRoute(hash) {
    return (hash || "#/").replace(/^#\/?/, "").replace(/^\/+/, "");
  }

  function toast(message) {
    const el = document.getElementById("toast");
    if (!el) return;
    el.textContent = message;
    el.classList.add("show");
    setTimeout(() => el.classList.remove("show"), 1800);
  }

  function escapeHtml(value) {
    return String(value).replace(/[&<>"']/g, ch => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[ch]));
  }

  async function loadApiData() {
    try {
      const [
        system,
        user,
        forwards,
        userDeviceGroups,
        adminDeviceGroups,
        adminUsers,
        userGroups,
        plans,
        orders,
        redeem,
        afflog,
        devices,
        nodeStatus,
        onlineIps,
        healthChecks,
        overview,
      ] = await Promise.all([
        apiGet("/api/v1/system/info"),
        apiGet("/api/v1/user/info"),
        apiGet("/api/v1/user/forward?page=1&size=20"),
        apiGet("/api/v1/user/devicegroup"),
        apiGet("/api/v1/admin/devicegroup"),
        apiGet("/api/v1/admin/user?page=1&size=20"),
        apiGet("/api/v1/admin/usergroup"),
        apiGet("/api/v1/admin/shop/plan"),
        apiGet("/api/v1/admin/shop/order?page=1&size=20"),
        apiGet("/api/v1/admin/shop/redeem?page=1&size=20"),
        apiGet("/api/v1/admin/aff/log?page=1&size=20"),
        apiGet("/api/v1/devices?page=1&size=50"),
        apiGet("/api/v1/system/node/status?page=1&size=50"),
        apiGet("/api/v1/online-ips?page=1&size=50"),
        apiGet("/api/v1/health-checks?page=1&size=50"),
        apiGet("/api/v1/dashboard/overview"),
      ]);

      state.system = system;
      state.user = user;
      state.overview = overview;
      const healthIndex = buildHealthIndex(listItems(healthChecks));
      rows.rules = listItems(forwards).map(rule => [
        escapeHtml(rule.name),
        escapeHtml(rule.entry),
        escapeHtml(rule.exit),
        escapeHtml(rule.strategy || "fallback"),
        escapeHtml(rule.currentConnections || 0),
        escapeHtml(rule.todayTraffic || "0.00 GiB"),
        escapeHtml(rule.errorCount || 0),
        escapeHtml(formatTime(rule.lastHitAt)),
        badge(escapeHtml(rule.status), statusTone(rule.status)),
        healthBadgeForTarget(healthIndex, "rule", rule.id),
        actions(["同步", rule.status === "已暂停" ? "启动" : "暂停", "删除"]),
      ]);
      rows.userDeviceGroups = listItems(userDeviceGroups).map(group => [
        escapeHtml(group.sort || group.id),
        escapeHtml(group.name),
        escapeHtml(group.type),
        escapeHtml(group.usedTraffic),
        escapeHtml(group.onlineDevices),
        escapeHtml(group.remark || ""),
        actions(["编辑", "设备", "删除"]),
      ]);
      rows.adminDeviceGroups = listItems(adminDeviceGroups).map(group => [
        escapeHtml(group.sort || group.id),
        escapeHtml(group.name),
        escapeHtml(group.userGroupId || group.userGroupID || "#1"),
        escapeHtml(group.type),
        escapeHtml(group.displayAddress || "-"),
        `${escapeHtml(group.multiplier || 1)}x`,
        escapeHtml(group.usedTraffic),
        escapeHtml(group.onlineDevices),
        escapeHtml(group.remark || ""),
        actions(["编辑", "设备", "高级"]),
      ]);
      rows.devices = listItems(devices).map(device => [
        escapeHtml(device.id),
        escapeHtml(device.name),
        escapeHtml(device.groupName || `#${device.groupId || ""}`),
        escapeHtml(device.type),
        badge(escapeHtml(deviceStatusText(device.status)), statusTone(device.status)),
        healthBadgeForTarget(healthIndex, "device", device.id),
        agentBadge(device),
        escapeHtml(device.configVersion || 1),
        escapeHtml(device.address || "-"),
        escapeHtml(device.region || "-"),
        escapeHtml(device.connectionCount || 0),
        `${escapeHtml(device.latencyMs || 0)}ms`,
        escapeHtml(device.load || "0%"),
        escapeHtml(formatTime(device.lastHeartbeat || device.lastSeen)),
        deviceActions(device),
      ]);
      rows.nodeStatus = listItems(nodeStatus).map(device => [
        escapeHtml(device.name),
        escapeHtml(device.groupName || `#${device.groupId || ""}`),
        escapeHtml(device.type),
        badge(escapeHtml(deviceStatusText(device.status)), statusTone(device.status)),
        healthBadgeForTarget(healthIndex, "device", device.id),
        escapeHtml(device.connectionCount || 0),
        `${escapeHtml(device.latencyMs || 0)}ms`,
        escapeHtml(device.load || "0%"),
        escapeHtml(formatTime(device.lastHeartbeat || device.lastSeen)),
        actions(["详情"]),
      ]);
      rows.lookingGlass = listItems(nodeStatus).map(device => [
        escapeHtml(device.name),
        escapeHtml(device.type),
        badge(deviceStatusText(device.status), statusTone(device.status)),
      ]);
      rows.onlineIps = listItems(onlineIps).map(item => [
        escapeHtml(`${item.sourceIp}${item.sourcePort ? ":" + item.sourcePort : ""}`),
        escapeHtml(item.ruleName || `#${item.ruleId || ""}`),
        escapeHtml(item.entryDeviceName || `#${item.entryDeviceId || ""}`),
        escapeHtml(item.protocol || "TCP"),
        badge(escapeHtml(realIPSourceText(item.realIpSource)), item.realIpSource && item.realIpSource.includes("proxy_protocol") ? "ok" : "warn"),
        escapeHtml(item.connectionCount || 0),
        escapeHtml(item.country || "-"),
        escapeHtml(formatTime(item.lastActiveAt)),
        actions([item.status === "active" ? "关闭" : "详情"]),
      ]);
      rows.healthChecks = listItems(healthChecks).map(item => [
        escapeHtml(item.name),
        `${escapeHtml(targetTypeText(item.targetType))} ${escapeHtml(item.targetName || `#${item.targetId || ""}`)}`,
        escapeHtml(String(item.protocol || "tcp").toUpperCase()),
        escapeHtml(`${item.host || "-"}${item.port ? ":" + item.port : ""}${item.path && item.path !== "/" ? item.path : ""}`),
        `${escapeHtml(item.intervalSec || 0)}s`,
        `${escapeHtml(item.timeoutSec || 0)}s`,
        badge(escapeHtml(healthStatusText(item.status)), statusTone(item.status)),
        item.lastLatencyMs ? `${escapeHtml(item.lastLatencyMs)}ms` : "-",
        escapeHtml(formatTime(item.lastCheckedAt)),
        escapeHtml(item.lastError || ""),
        healthActions(item),
      ]);
      rows.users = listItems(adminUsers).map(item => [
        escapeHtml(item.uid || item.id),
        escapeHtml(item.username),
        escapeHtml(item.planExpire),
        `${escapeHtml(item.trafficUsed)} / ${escapeHtml(item.trafficTotal)}`,
        escapeHtml(item.plan),
        escapeHtml(item.userGroup),
        escapeHtml(item.maxRules),
        escapeHtml(item.walletBalance),
        actions(["编辑", "规则", "余额"]),
        escapeHtml(item.remark || ""),
      ]);
      rows.userGroups = listItems(userGroups).map(group => [
        escapeHtml(group.sort || group.id),
        `#${escapeHtml(group.id)}`,
        escapeHtml(group.name),
        escapeHtml(group.userCount),
        actions(["编辑", "删除"]),
      ]);
      rows.plans = listItems(plans).map(plan => [
        escapeHtml(plan.sort || plan.id),
        escapeHtml(plan.name),
        escapeHtml(plan.type),
        escapeHtml(plan.userGroup),
        escapeHtml(plan.traffic),
        escapeHtml(plan.ruleLimit),
        escapeHtml(plan.price),
        badge(plan.hidden ? "是" : "否", plan.hidden ? "warn" : "ok"),
        actions(["编辑", "隐藏", "删除"]),
      ]);
      rows.userOrders = listItems(orders).map(order => orderRow(order, false));
      rows.adminOrders = listItems(orders).map(order => orderRow(order, true));
      rows.redeem = listItems(redeem).map(code => [
        escapeHtml(code.id),
        escapeHtml(code.code),
        escapeHtml(code.plan),
        escapeHtml(code.discount),
        escapeHtml(code.remain),
        actions(["复制", "删除"]),
      ]);
      rows.afflog = listItems(afflog).map(item => [
        escapeHtml(item.user),
        escapeHtml(item.createdAt),
        escapeHtml(item.info),
        escapeHtml(item.amount),
        escapeHtml(item.type),
        badge(escapeHtml(item.status), statusTone(item.status)),
        actions(["详情", "删除"]),
      ]);
      state.apiLoaded = true;
      state.apiError = "";
      render();
    } catch (error) {
      state.apiError = error && error.message ? error.message : String(error);
      console.warn("API data fallback:", state.apiError);
    }
  }

  async function apiGet(path) {
    const response = await fetch(path, { headers: { "Accept": "application/json" }, credentials: "include" });
    if (!response.ok) {
      throw new Error(`${path} ${response.status}`);
    }
    const payload = await response.json();
    if (payload && Object.prototype.hasOwnProperty.call(payload, "success")) {
      if (!payload.success) {
        throw new Error(payload.message || path);
      }
      return payload.data;
    }
    return payload;
  }

  async function apiSend(path, body, method) {
    const response = await fetch(path, {
      method: method || "POST",
      credentials: "include",
      headers: { "Accept": "application/json", "Content-Type": "application/json" },
      body: JSON.stringify(body || {}),
    });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok || (payload && payload.success === false)) {
      throw new Error((payload && payload.message) || `${path} ${response.status}`);
    }
    return payload.data;
  }

  async function login() {
    try {
      const username = formValue("loginUsername") || "admin";
      const password = formValue("loginPassword");
      const data = await apiSend("/api/v1/auth/login", { username, password });
      state.user = data.user;
      state.authed = true;
      localStorage.setItem("repleypass-auth", "1");
      await loadApiData();
      go("");
    } catch (error) {
      toast(error.message || "登录失败");
    }
  }

  async function logout() {
    try {
      await apiSend("/api/v1/auth/logout", {});
    } catch (error) {
      console.warn("logout failed:", error);
    }
  }

  async function saveModal() {
    try {
      if (state.modalKind === "rule") {
        await createRule();
        return;
      }
      if (state.modalKind === "batch") {
        await importRules();
        return;
      }
      if (state.modalKind === "deviceGroup") {
        await createDeviceGroup();
        return;
      }
      if (state.modalKind === "device") {
        await createDevice();
        return;
      }
      if (state.modalKind === "onlineIp") {
        await createOnlineIP();
        return;
      }
      if (state.modalKind === "healthCheck") {
        await createHealthCheck();
        return;
      }
      toast("演示保存成功");
    } catch (error) {
      toast(error.message || "保存失败");
    }
  }

  async function createRule() {
    await apiSend("/api/v1/user/forward", {
      name: formValue("ruleName"),
      entryGroupId: Number(formValue("ruleEntryGroupId") || 0),
      entryGroupName: formValue("ruleEntryGroupName"),
      listenHost: formValue("ruleListenHost"),
      listenPort: formValue("ruleListenPort"),
      exitGroupId: Number(formValue("ruleExitGroupId") || 0),
      exitGroupName: formValue("ruleExitGroupName"),
      targetHost: formValue("ruleTargetHost"),
      targetPort: formValue("ruleTargetPort"),
      strategy: formValue("ruleStrategy") || "fallback",
      usedTraffic: "0.00 GiB",
      todayTraffic: "0.00 GiB",
      status: "未同步",
      syncStatus: "未同步",
      group: formValue("ruleGroup"),
      protocol: formValue("ruleProtocol"),
      proxyProtocol: formValue("ruleProxy"),
      proxyProtocolMode: formValue("ruleProxyMode") || "send",
      owner: (state.user && state.user.username) || "admin",
      remark: formValue("ruleRemark"),
    });
    state.modal = null;
    state.modalKind = "";
    await loadApiData();
    toast("规则已保存");
  }

  async function importRules() {
    const parsed = JSON.parse(formValue("batchRulesJson"));
    await apiSend("/api/v1/rules/batch-import", Array.isArray(parsed) ? { rules: parsed } : parsed);
    state.modal = null;
    state.modalKind = "";
    await loadApiData();
    toast("规则已导入");
  }

  async function exportRules() {
    const data = await apiGet("/api/v1/rules/batch-export");
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `repleypass-rules-${Date.now()}.json`;
    link.click();
    URL.revokeObjectURL(url);
    toast("规则已导出");
  }

  async function createDeviceGroup() {
    await apiSend("/api/v1/admin/devicegroup", {
      sort: Number(formValue("groupSort") || 0),
      name: formValue("groupName"),
      userGroupId: formValue("groupUserGroupId") || "#1",
      type: formValue("groupType") || "出口",
      displayAddress: formValue("groupDisplayAddress") || "-",
      multiplier: Number(formValue("groupMultiplier") || 1),
      usedTraffic: "0.00 GiB",
      onlineDevices: 0,
      remark: formValue("groupRemark"),
    });
    state.modal = null;
    state.modalKind = "";
    await loadApiData();
    toast("设备组已保存");
  }

  async function createDevice() {
    await apiSend("/api/v1/devices", {
      name: formValue("deviceName"),
      groupId: Number(formValue("deviceGroupId") || 0),
      groupName: formValue("deviceGroupName"),
      type: formValue("deviceType") || "出口",
      status: formValue("deviceStatus") || "offline",
      address: formValue("deviceAddress"),
      region: formValue("deviceRegion"),
      version: formValue("deviceVersion") || "edge-0.1.0",
      load: "0%",
      latencyMs: 0,
      connectionCount: 0,
      inboundTraffic: "0.00 GiB",
      outboundTraffic: "0.00 GiB",
      remark: formValue("deviceRemark"),
    });
    state.modal = null;
    state.modalKind = "";
    await loadApiData();
    toast("设备已保存");
  }

  async function createOnlineIP() {
    await apiSend("/api/v1/online-ips", {
      sourceIp: formValue("onlineSourceIp"),
      sourcePort: Number(formValue("onlineSourcePort") || 0),
      ruleId: Number(formValue("onlineRuleId") || 0),
      ruleName: formValue("onlineRuleName"),
      entryDeviceId: Number(formValue("onlineEntryDeviceId") || 0),
      entryDeviceName: formValue("onlineEntryDeviceName"),
      entryGroupName: formValue("onlineEntryGroupName"),
      protocol: formValue("onlineProtocol") || "TCP",
      realIpSource: formValue("onlineRealIpSource") || "connection_log",
      connectionCount: Number(formValue("onlineConnectionCount") || 1),
      country: formValue("onlineCountry"),
      status: "active",
    });
    state.modal = null;
    state.modalKind = "";
    await loadApiData();
    toast("在线连接已记录");
  }

  async function createHealthCheck() {
    await apiSend("/api/v1/health-checks", {
      name: formValue("checkName"),
      targetType: formValue("checkTargetType") || "device",
      targetId: Number(formValue("checkTargetId") || 0),
      targetName: formValue("checkTargetName"),
      protocol: formValue("checkProtocol") || "tcp",
      host: formValue("checkHost"),
      port: Number(formValue("checkPort") || 0),
      path: formValue("checkPath"),
      intervalSec: Number(formValue("checkIntervalSec") || 60),
      timeoutSec: Number(formValue("checkTimeoutSec") || 5),
      status: "unknown",
      remark: formValue("checkRemark"),
    });
    state.modal = null;
    state.modalKind = "";
    await loadApiData();
    toast("健康检查已保存");
  }

  async function runHealthCheck(id) {
    if (!id) return;
    await apiSend(`/api/v1/health-checks/${id}/run`, {});
    await loadApiData();
    toast("探针已执行");
  }

  async function showHealthResults(id, name) {
    if (!id) return;
    const data = await apiGet(`/api/v1/health-checks/${id}/results?page=1&size=10`);
    const rows = listItems(data).map(item => [
      badge(escapeHtml(healthStatusText(item.status)), statusTone(item.status)),
      item.latencyMs ? `${escapeHtml(item.latencyMs)}ms` : "-",
      escapeHtml(formatTime(item.checkedAt)),
      escapeHtml(item.failureReason || ""),
    ]);
    state.modalKind = "healthResults";
    state.modal = [
      `探针结果 - ${escapeHtml(name || "#" + id)}`,
      table(["状态", "延迟", "检查时间", "失败原因"], rows),
    ];
    render();
  }

  async function generateAgentToken(id, name) {
    if (!id) return;
    const data = await apiSend(`/api/v1/devices/${id}/agent-token`, {});
    state.modalKind = "agentToken";
    state.modal = [
      `Agent Token - ${escapeHtml(name || "#" + id)}`,
      `
        <div class="info-grid">
          ${info("设备", escapeHtml((data.device && data.device.name) || name || id))}
          ${info("配置版本", escapeHtml((data.device && data.device.configVersion) || "-"))}
          ${info("Token", `<code>${escapeHtml(data.token)}</code>`)}
          ${info("注册接口", "<code>POST /api/v1/agent/register</code>")}
          ${info("心跳接口", "<code>POST /api/v1/agent/heartbeat</code>")}
          ${info("拉取配置", "<code>GET /api/v1/agent/config</code>")}
        </div>
        <p class="muted" style="margin-top:14px">Token 只在本次生成后显示一次，节点端使用 Authorization: Bearer 发送。</p>
      `,
    ];
    await loadApiData();
    render();
  }

  function formValue(id) {
    const el = document.getElementById(id);
    return el ? el.value.trim() : "";
  }

  function listItems(value) {
    if (!value) return [];
    if (Array.isArray(value)) return value;
    if (Array.isArray(value.items)) return value.items;
    return [];
  }

  function statusTone(status) {
    if (status === "正常" || status === "已记录" || status === "online" || status === "healthy") return "ok";
    if (status === "未同步" || status === "待支付" || status === "alert" || status === "warning" || status === "unknown") return "warn";
    if (status === "同步失败" || status === "failed") return "bad";
    return "off";
  }

  function deviceStatusText(status) {
    const map = { online: "在线", offline: "离线", disabled: "禁用", alert: "告警" };
    return map[status] || status || "离线";
  }

  function realIPSourceText(source) {
    const map = {
      proxy_protocol_v2: "Proxy Protocol v2",
      proxy_protocol_v1: "Proxy Protocol v1",
      connection_log: "连接日志",
      fallback: "回退识别",
    };
    return map[source] || source || "连接日志";
  }

  function healthStatusText(status) {
    const map = { healthy: "健康", warning: "告警", failed: "失败", unknown: "未知", disabled: "禁用" };
    return map[status] || status || "未知";
  }

  function targetTypeText(type) {
    const map = { device: "设备", rule: "规则", service: "服务" };
    return map[type] || type || "设备";
  }

  function agentBadge(device) {
    if (device.agentRegisteredAt) return badge("已注册", "ok");
    return badge("未注册", "off");
  }

  function deviceActions(device) {
    return `<div class="toolbar"><button class="small" data-action="noop">编辑</button><button class="small" data-action="generate-agent-token" data-id="${escapeHtml(device.id)}" data-name="${escapeHtml(device.name)}">Token</button><button class="small" data-action="noop">${device.enabled ? "禁用" : "启用"}</button><button class="small danger" data-action="noop">删除</button></div>`;
  }

  function buildHealthIndex(items) {
    return items.reduce((index, item) => {
      const key = `${item.targetType || "device"}:${item.targetId || 0}`;
      const current = index[key];
      if (!current || healthRank(item.status) > healthRank(current.status)) {
        index[key] = item;
      }
      return index;
    }, {});
  }

  function healthBadgeForTarget(index, type, id) {
    const item = index[`${type}:${id || 0}`];
    if (!item) return badge("未配置", "off");
    return badge(healthStatusText(item.status), statusTone(item.status));
  }

  function healthRank(status) {
    const ranks = { failed: 4, warning: 3, unknown: 2, healthy: 1, disabled: 0 };
    return ranks[status] || 0;
  }

  function healthActions(item) {
    return `<div class="toolbar"><button class="small" data-action="run-health-check" data-id="${escapeHtml(item.id)}">执行</button><button class="small" data-action="show-health-results" data-id="${escapeHtml(item.id)}" data-name="${escapeHtml(item.name)}">结果</button><button class="small danger" data-action="noop">删除</button></div>`;
  }

  function formatTime(value) {
    if (!value) return "-";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString("zh-CN", { hour12: false });
  }

  function orderRow(order, includeUser) {
    const row = [
      escapeHtml(order.id),
    ];
    if (includeUser) row.push(escapeHtml(order.user));
    row.push(
      escapeHtml(order.createdAt),
      escapeHtml(order.paidAt || "-"),
      escapeHtml(order.info),
      escapeHtml(order.amount),
      escapeHtml(order.type),
      badge(escapeHtml(order.status), statusTone(order.status)),
      actions(["详情"])
    );
    return row;
  }
})();
