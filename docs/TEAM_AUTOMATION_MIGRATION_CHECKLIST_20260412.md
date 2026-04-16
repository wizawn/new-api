# Team Automation → NewAPI 迁移清单（2026-04-12）

## 1. 真实边界

这份清单只对应本地源码改造结果，**不代表你的远端服务器已经完成部署**。

当前可交付的是：

- 已修改好的 NewAPI 源码目录：`/root/work/new-api-official`
- 源码压缩包：`/root/work/new-api-official-team-automation-adapter-20260412.zip`
- 统一补丁：`/root/work/new-api-official-team-automation-adapter-20260412.patch`

## 2. 推荐迁移方式

### 方式 A：优先推荐，按 patch 应用

适用条件：

- 你远端 NewAPI 源码版本与本地 `new-api-official` 接近
- 远端仓库没有大规模自定义改动

命令示例：

```bash
cd /path/to/your/new-api
cp /tmp/new-api-official-team-automation-adapter-20260412.patch ./
git apply --check new-api-official-team-automation-adapter-20260412.patch
git apply new-api-official-team-automation-adapter-20260412.patch
```

如果 `git apply --check` 失败：

- 说明远端代码版本差异过大；
- 不要强行套 patch；
- 改用“方式 B：逐文件迁移”。

### 方式 B：逐文件迁移

适用条件：

- 远端 NewAPI 已做过较多自定义
- patch 存在冲突

把下列文件从本地改造版复制到远端相同路径后，再人工处理冲突。

## 3. 必迁移文件清单

### 后端新增文件

- `controller/team_automation.go`
- `controller/team_automation_test.go`
- `service/team_automation.go`
- `service/team_automation_test.go`

### 后端修改文件

- `model/option.go`
- `router/api-router.go`

### 前端新增文件

- `web/src/components/settings/TeamAutomationSetting.jsx`
- `web/src/helpers/teamAutomation.js`
- `web/src/helpers/lobeIconsRegistry.js`
- `web/src/pages/TeamAutomation/index.jsx`
- `web/src/pages/TeamAutomation/useTeamAutomationPage.js`
- `web/src/pages/TeamAutomation/sections.jsx`
- `web/src/pages/TeamAutomation/AccountModal.jsx`
- `web/src/pages/TeamAutomation/constants.js`
- `web/src/pages/TeamAutomation/utils.jsx`

### 前端修改文件

- `web/src/App.jsx`
- `web/src/components/layout/SiderBar.jsx`
- `web/src/components/settings/personal/cards/NotificationSettings.jsx`
- `web/src/helpers/index.js`
- `web/src/pages/Home/index.jsx`
- `web/src/helpers/render.jsx`
- `web/src/hooks/common/useSidebar.js`
- `web/src/pages/Setting/index.jsx`
- `web/src/pages/Setting/Operation/SettingsSidebarModulesAdmin.jsx`
- `web/package.json`
- `web/vite.config.js`

### 文档 / 脚本文件

- `docs/TEAM_AUTOMATION_NEWAPI_ADAPTER_20260412.md`
- `docs/TEAM_AUTOMATION_MIGRATION_CHECKLIST_20260412.md`
- `scripts/team-automation-adapter-smoke.sh`

## 4. 迁移后必须确认的配置点

### NewAPI Option 默认键

确认以下键已存在：

- `TeamAutomationEnabled`
- `TeamAutomationBaseURL`
- `TeamAutomationUsername`
- `TeamAutomationPasswordSecret`
- `TeamAutomationTimeoutSeconds`
- `TeamAutomationVerifySSLEnabled`

对应位置：

- `model/option.go`

额外约束：

- `TeamAutomationBaseURL` 必须使用 `http` / `https`
- `TeamAutomationTimeoutSeconds` 必须在 `5~300` 之间

### NewAPI 后端路由

确认以下路由已注册：

- `GET /api/team-automation/settings`
- `POST /api/team-automation/settings/test-connection`
- `GET /api/team-automation/health`
- `GET /api/team-automation/stats`
- `GET /api/team-automation/cache/stats`
- `GET /api/team-automation/accounts`
- `POST /api/team-automation/accounts`
- `GET /api/team-automation/accounts/stats`
- `GET /api/team-automation/accounts/export/csv`
- `POST /api/team-automation/accounts/import/csv`
- `GET /api/team-automation/accounts/:email`
- `PUT /api/team-automation/accounts/:email`
- `DELETE /api/team-automation/accounts/:email`
- `POST /api/team-automation/run`
- `GET /api/team-automation/run/history`
- `GET /api/team-automation/run/:id`
- `GET /api/team-automation/run/:id/logs`
- `POST /api/team-automation/run/:id/stop`

对应位置：

- `router/api-router.go`

## 5. 迁移后的建议验证顺序

### 第一步：gofmt + 后端测试

```bash
cd /path/to/your/new-api
gofmt -w controller/team_automation.go controller/team_automation_test.go service/team_automation.go service/team_automation_test.go router/api-router.go
./scripts/team-automation-adapter-smoke.sh
```

补充说明：

- `./scripts/team-automation-adapter-smoke.sh` 只验证 Team Automation 相关功能；
- 它**不代表**整个仓库已经完成生产构建。

### 第二步：前端依赖

这次改造额外依赖了：

- `antd`

因此建议执行：

```bash
cd /path/to/your/new-api/web
npm install --legacy-peer-deps
```

### 第三步：前端构建

```bash
cd /path/to/your/new-api/web
NODE_OPTIONS=--max-old-space-size=4096 npm run build
```

如果你的前端包体和插件较多，建议：

- 至少 4 GB 可用内存
- 更稳妥是 6~8 GB

### 第四步：后台人工验收

在 NewAPI 后台依次验证：

1. `系统设置 -> Team 自动化连接器`
   - 保存配置
   - 测试连接
2. `Team 自动化`
   - 拉取账号列表
   - 新增账号
   - 查看详情
   - 编辑状态
   - 删除账号
   - CSV 导入
   - CSV 导出
   - 创建执行任务
   - 查看历史 / 详情 / 日志
   - 停止任务

## 6. 迁移后的人工配置步骤

在 NewAPI 后台 Root 设置中配置：

- `TeamAutomationEnabled`
- `TeamAutomationBaseURL`
- `TeamAutomationUsername`
- `TeamAutomationPasswordSecret`
- `TeamAutomationTimeoutSeconds`
- `TeamAutomationVerifySSLEnabled`

建议：

- 先在内网或 VPN 环境验证；
- 不要一开始就把 Team Automation / OpenClaw 直接裸暴露公网。

## 7. 当前未完成的能力

以下不是漏做，而是远端 Team Automation API 当前未提供对应接口，所以不能诚实宣称已做完：

- 任务暂停
- 任务删除
- 历史任务清理
- 任务级权限更细粒度拆分
- 远端 NewAPI 实例真实生产 E2E
- 当前机器上的完整 `vite build` 产物确认（最新一次已被系统以退出码 137 终止）

## 8. 风险提示

1. 如果你远端 NewAPI 分支与官方仓库差异较大，patch 很可能冲突。
2. 如果远端服务器内存不足，前端 `vite build` 可能继续卡住或超时。
3. 当前适配是“NewAPI 原生入口 + 远端 Team Automation 代理”，**不是把 Python 自动化执行器改写成 Go 内嵌模块**。
4. OpenClaw 仍然应部署在 Team Automation 服务后面，不建议直接由公网访问。
