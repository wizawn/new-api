# Team Automation → NewAPI 全量交接文档（2026-04-12）

## 1. 交付结论

### 已完成

- 已基于 **NewAPI 官方源码** 完成 Team Automation 原生接入改造。
- 已实现 **可插拔 OpenClaw / Team Automation 连接器架构**：
  - NewAPI 只负责原生管理 UI、权限、配置、后端代理；
  - Team Automation / OpenClaw 继续外置部署；
  - 后期可按你自己的节奏开放公网，当前源码已为此预留连接器配置位。
- 已补齐本地源码交付、迁移文档、补丁、压缩包、smoke 脚本、测试。

### 未完成且不能虚假宣称已完成

- **未在你另一台服务器上的真实 NewAPI 实例完成部署**。
- **未完成远端真实 E2E / Playwright / 多机联调**。
- **当前机器上 `web/npm run build` 仍未成功产出 `web/dist`**。

## 2. 当前架构

```text
NewAPI(Admin UI + Go Proxy)
        |
        v
Team Automation HTTP API (remote/pluggable)
        |
        +-- OpenClaw
        +-- Gmail IMAP
        +-- Cloudflare API
        +-- Redeem Sites
        +-- Session Conversion / NewAPI Import
```

## 3. 已落地文件

### 后端新增

- `controller/team_automation.go`
- `controller/team_automation_test.go`
- `service/team_automation.go`
- `service/team_automation_test.go`

### 后端修改

- `model/option.go`
- `router/api-router.go`

### 前端新增

- `web/src/components/settings/TeamAutomationSetting.jsx`
- `web/src/helpers/teamAutomation.js`
- `web/src/helpers/lobeIconsRegistry.js`
- `web/src/pages/TeamAutomation/index.jsx`
- `web/src/pages/TeamAutomation/useTeamAutomationPage.js`
- `web/src/pages/TeamAutomation/sections.jsx`
- `web/src/pages/TeamAutomation/AccountModal.jsx`
- `web/src/pages/TeamAutomation/constants.js`
- `web/src/pages/TeamAutomation/utils.jsx`

### 前端修改

- `web/src/App.jsx`
- `web/src/components/layout/SiderBar.jsx`
- `web/src/components/settings/personal/cards/NotificationSettings.jsx`
- `web/src/helpers/index.js`
- `web/src/helpers/render.jsx`
- `web/src/hooks/common/useSidebar.js`
- `web/src/pages/Home/index.jsx`
- `web/src/pages/Setting/index.jsx`
- `web/src/pages/Setting/Operation/SettingsSidebarModulesAdmin.jsx`
- `web/package.json`
- `web/vite.config.js`

### 工程化 / 文档新增

- `scripts/team-automation-adapter-smoke.sh`
- `docs/TEAM_AUTOMATION_NEWAPI_ADAPTER_20260412.md`
- `docs/TEAM_AUTOMATION_MIGRATION_CHECKLIST_20260412.md`
- `docs/TEAM_AUTOMATION_FULL_HANDOFF_20260412.md`

## 4. 功能范围

### Root 设置页

- Team Automation 连接器启用/禁用
- Base URL
- Basic Auth 用户名/密码
- TLS 校验开关
- 超时配置
- 测试连接
- 密码保留/清空逻辑

### Admin 页面

- 健康检查
- 缓存状态
- 统计概览
- 启动任务
- 停止任务
- 执行历史
- 执行详情 / 日志
- 账号列表 / 筛选
- 新增账号
- 账号详情
- 账号状态编辑
- 删除账号
- CSV 导入 / 导出

## 5. 本轮进一步优化

### 5.1 TeamAutomation 页面模块化

原始状态：

- `web/src/pages/TeamAutomation/index.jsx` 约 **1310 行**，可维护性差。

当前状态：

- `index.jsx`：页面装配层
- `useTeamAutomationPage.js`：状态与副作用
- `sections.jsx`：主要展示区块
- `AccountModal.jsx`：账号弹窗
- `constants.js` / `utils.jsx`：常量与通用显示组件

### 5.2 前端构建侧减负尝试

已做但仍未完全解决：

- `code-inspector-plugin` 改为仅 `serve` 时启用；
- `@lobehub/icons` 根入口导入替换为本地生成的深导入注册表 `web/src/helpers/lobeIconsRegistry.js`；
- 为 `@visactor/*` 增加独立 manual chunk；
- 这些动作减少了无关入口放大问题，但 **没有在当前机器上彻底跑通完整 build**。

### 5.3 测试补强

新增控制器测试覆盖：

- 配置接口不泄漏密码
- CSV 导出文件返回
- CSV 导出本地筛选
- CSV 缺列错误
- CSV 导入 multipart 转发
- CSV 导入缺失文件错误

## 6. 已执行验证

### 已通过

```bash
cd /root/work/new-api-official
./scripts/team-automation-adapter-smoke.sh
```

该脚本实际包含：

```bash
go test ./service ./controller -run TeamAutomation -count=1
cd web && npx esbuild src/pages/TeamAutomation/index.jsx src/components/settings/TeamAutomationSetting.jsx src/helpers/teamAutomation.js --bundle --format=esm --external:* --outdir=/tmp/newapi-team-automation-smoke
```

### 补丁可应用性已通过

```bash
git apply --check /root/work/new-api-official-team-automation-adapter-20260412.patch
```

结果：

```text
PATCH_CHECK_OK
```

## 7. 真实失败边界

### 7.1 仓库级 Go 空跑

```bash
cd /root/work/new-api-official
go test ./... -run '^$' -count=1
```

真实结果：

- 仍会因为 `main.go` 依赖 `web/dist` 而失败；
- 错误：`main.go:37:12: pattern web/dist: no matching files found`

### 7.2 前端完整 build

```bash
cd /root/work/new-api-official/web
NODE_OPTIONS=--max-old-space-size=3072 npm run build
```

真实结果：

- 进入 `vite build`；
- 长时间停留在 `transforming...`；
- 最新一次最终被系统以 **退出码 137 (`Killed`)** 终止；
- 当时 Node RSS 约 **2.08 GiB**，主机 swap 基本耗尽；
- 所以不能诚实宣称 `web/dist` 已成功产出。

## 8. 交付物路径

- 源码目录：`/root/work/new-api-official`
- 压缩包：`/root/work/new-api-official-team-automation-adapter-20260412.zip`
- 补丁：`/root/work/new-api-official-team-automation-adapter-20260412.patch`

## 9. 你接下来应如何使用

### 方案 A：优先推荐

- 下载 zip
- 在另一台服务器上的 NewAPI 源码仓库中对照迁移
- 先运行：

```bash
./scripts/team-automation-adapter-smoke.sh
```

- 再在更大内存环境执行：

```bash
cd web
npm install --legacy-peer-deps
NODE_OPTIONS=--max-old-space-size=4096 npm run build
```

### 方案 B：按 patch 应用

```bash
git apply --check new-api-official-team-automation-adapter-20260412.patch
git apply new-api-official-team-automation-adapter-20260412.patch
```

## 10. 必须继续由你在远端完成的事项

1. 真实部署到你的 NewAPI 服务器
2. 配置 `TeamAutomationBaseURL`
3. 配置公网/反代/TLS/Basic Auth/IP 白名单
4. 使用你真实远端 Team Automation 服务做联调
5. 做最终 E2E / Playwright 验收

## 11. 最终坦诚结论

### 我已经完成的

- 本地 NewAPI 官方源码层面的 Team Automation 原生适配
- OpenClaw 可插拔远端连接器准备
- 测试、补丁、压缩包、迁移文档、完整交接文档

### 我没有完成且不能谎称完成的

- 你的远端部署
- 你的远端真实 E2E
- 当前机器上的完整 Vite 生产构建通过
