# OVH 隔离部署

核心沿用 QuantumNous/new-api v1.0.0-rc.49（48a6c87f9e7757fdc58e5be9bcb4f8281ad0dfd0），未修改业务源码。官方镜像来源和摘要在 image-lock.json；本机 .env 使用导入后 docker image inspect 返回的不可变镜像 ID。部署位置 /opt/new-api-router。

## 运行边界

- 使用已有隔离 Docker socket /run/redeemai-rehearsal-docker/docker.sock，网络命名空间 /run/netns/redeemai-rehearsal。
- 独立桥接网络 newapi-out（10.206.60.0/24）和 newapi-db（10.206.61.0/24）。数据库与 Redis 不发布端口。network.py 只维护已有防火墙中的两条自有链，禁止连接其他私网。
- 外部入口 router.cdkey.si → Cloudflare 代理、Strict TLS、禁用边缘缓存 → 雷池站点 17 / 2053 TLS → 命名空间回环 3000。
- New API 的 TRUSTED_PROXIES 仅包含回环、10.206.60.1 与 Cloudflare 官方 API 返回的网段；配合雷池现有 X-Forwarded-For 追加机制解析真实客户端地址。
- 注册关闭，管理员 yanling。初始凭据只保存在服务器 root 可读的 admin.private.json；部署密钥保存在 .env，均不提交 Git。
- new_api_tools 为独立容器，前端沿用 cafb9ee 镜像，后端为 wizawn/new_api_tools 的 feature/optional-background-writes 分支。AUTO_ENSURE_INDEXES=false、ENFORCE_IP_RECORDING=false；数据库角色 newapi_tools 有显式业务读写权限，无 DDL 或表所有权。后台主动开索引、修改用户 IP 记录均关闭，管理页面的显式写操作仍保留。

## 运维入口

在服务器上使用：

```sh
sudo systemctl status new-api-router.service
sudo docker --host unix:///run/redeemai-rehearsal-docker/docker.sock compose -f /opt/new-api-router/compose.yaml --profile tools ps
```

旁路管理页仅允许 SSH 访问；本地建立隧道后访问 http://127.0.0.1:1145：

```sh
ssh -N -L 1145:127.0.0.1:1145 ubuntu@192.99.233.83
```

其密码为服务器 .env 中 TOOLS_ADMIN_PASSWORD。new-api-router-tools.socket 仅监听宿主机 127.0.0.1:1145，不开放公网。

## 备份与更新

new-api-router-backup.timer 每日执行原生 pg_dump / pg_dumpall、工具 SQLite 在线备份，并复用已有 Google Drive 加密 Restic 仓库。暂存位于 /run，服务结束即清除；backup-last-success.json 记录已回读快照清单的结果。Redis 缓存和 GeoIP 可重建文件不纳入此备份。快照清单验证不等于恢复演练。

更新前取得新 release，核对 image-lock.json 与上游 revision，保留旧镜像和异地快照，更新本机 .env 中对应镜像 ID。先检查迁移兼容性，再仅重建本 Compose 项目的容器。不要使用全局 docker prune，也不要重启共享 Docker 守护进程。

分支保留官方核心与部署配置的边界；上游仓库为 upstream，个人 fork 为 origin。旁路工具补丁独立维护，不合并其他大型二开。

## 验证边界

已核对初始化、管理员登录、选项回读、四容器健康、旁路数据库连接、自动索引/IP 设置未变、TLS 回源和公网 API 入口。尚无上游模型渠道密钥，因此真实模型调用与端到端 SSE 未验证。

雷池当前许可证拒绝按站点设置 SSE、源 IP 与其他高级代理选项（返回 license required），未修改这些选项。官方 New API 的 relay/helper/common.go 自带 X-Accel-Buffering: no 与 Cache-Control: no-cache；雷池使用 HTTP/1.1 代理，Cloudflare 已对该主机名禁用缓存。真实模型流仍待提供渠道后验证。
