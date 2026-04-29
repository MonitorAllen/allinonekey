# All In One Key (allinonekey) 项目需求文档

## 1. 项目概述

All In One Key 是一个轻量级的 AI API Key 及个人账号密码管理系统。

核心定位是：**简单、高效、私密、可本地部署**。

系统用于统一管理多服务商 AI API Key、按池子分组、批量导入、额度巡检、个人账号密码保险箱、邀请码注册、多用户隔离与审计日志。

## 2. 产品原则

- **轻量化优先**：单体服务 + SQLite，不引入复杂中间件。
- **零知识优先**：敏感明文不落库，Master Key 不持久化。
- **本地部署优先**：默认面向个人或小团队自托管场景。
- **暗色极简 UI**：避免重型后台系统风格，界面要直接、干净。
- **模块独立刷新**：API Keys、Accounts、Audit Logs、Admin 模块互不串扰。
- **显式接口优先**：后端路由必须避免动态路径与静态路径冲突。
- **版本规范优先**：项目采用 SemVer 风格的 `MAJOR.MINOR.PATCH` 三段式版本号，初始阶段从 `0.0.0` 起步。

## 3. 技术选型

### 3.1 后端

- Go 1.25.0
- Gin
- GORM
- SQLite，驱动使用 `modernc.org/sqlite`
- Bearer Token 鉴权：登录后返回 AES-GCM sealed session token
- Argon2id Master Key 校验
- AES-256-GCM 敏感数据加密

### 3.2 前端

- Vue 3
- TypeScript
- Vite
- TailwindCSS v4
- Pinia
- Axios
- Lucide Vue Icons
- Bun 作为包管理与构建工具

### 3.3 部署

- Docker multi-stage build
- 前端构建镜像基于 `node:22-alpine`，再安装 Bun
- 后端构建镜像基于 `golang:1.25-alpine`
- Go 依赖下载使用 `GOPROXY=https://goproxy.cn,direct`
- 默认服务端口：`8080`
- 本地开发前端端口：`5173`

## 4. 安全架构

### 4.1 零知识架构

- 用户登录时输入 Username + Master Key。
- 后端只保存 Master Key 的 Argon2id verifier 与 salt。
- Master Key 明文不写入数据库。
- 敏感字段使用 AES-256-GCM 加密后落库。
- 解密只在用户已登录、请求携带有效 sealed session token 时发生。

### 4.2 当前会话设计

- 登录成功后后端签发 AES-GCM sealed session token。
- sealed session payload 包含当前会话所需的用户 ID、角色、Master Key 与过期时间，但客户端只能拿到 opaque token，不能 base64 读取 Master Key。
- sealed session 默认 24 小时过期。
- 本地 `make dev` / `make dev-server` 使用 Makefile 的开发占位 `JWT_SECRET` 与 `SESSION_SECRET`，生产与 Docker 启动必须显式设置强随机 `ALLINONEKEY_JWT_SECRET` 与 `ALLINONEKEY_SESSION_SECRET`。
- 后端 AuthMiddleware 解封 session 后把用户 ID、角色与 Master Key 写入请求上下文，用于加密新增数据或按需解密。
- `ALLINONEKEY_SESSION_SECRET` 是 sealed session 加密密钥来源；`ALLINONEKEY_JWT_SECRET` 保留为应用级兼容/启动守卫密钥。

### 4.3 敏感字段

必须加密存储：

- API Key 原文
- Account 密码

不得直接返回给列表接口：

- API Key 原文
- Account 密码明文
- Master Key
- Master Key verifier
- Salt

## 5. 数据模型

### 5.1 User

- `id`
- `username`
- `role`：`admin` / `user`
- `key_verifier`
- `salt`
- `created_at`
- `updated_at`

### 5.2 APIKey

- `id`
- `user_id`
- `provider`
- `pool_group`
- `key_name`
- `key_value`：AES-256-GCM 密文
- `base_url`
- `proxy_url`
- `quota_total`
- `quota_used`
- `quota_balance`
- `last_check`
- `status`
- `created_at`
- `updated_at`

### 5.3 Account

- `id`
- `user_id`
- `platform`
- `url`
- `account`
- `password`：AES-256-GCM 密文
- `totp_secret`：AES-256-GCM 密文
- `has_totp`
- `favicon_url`
- `created_at`
- `updated_at`

### 5.4 InvitationCode

- `id`
- `code`
- `created_by`
- `used_by`
- `is_used`
- `expires_at`
- `created_at`

### 5.5 AuditLog

- `id`
- `user_id`
- `action`
- `detail`
- `ip`
- `created_at`

## 6. 功能模块与状态

### 6.1 AI API Key 管理

- [x] 多服务商 API Key 管理。
- [x] Pool Group 分组展示。
- [x] Key 新增交互改为按需添加多行表单：每行包含自定义 Key Name 与 Key Value，不再使用“一行一个 Key”的 textarea 批量导入作为主入口。
- [x] Key 编辑能力完善，支持 provider / pool_group / key_name / base_url / proxy_url / status / key_value 更新。
- [x] Key 额度/健康详情展示完善：列表展示 status、quota_total、quota_used、quota_balance、last_check、base_url、proxy_url，并在健康探测后刷新。
- [x] API Keys 支持按模块导出 JSON/CSV，并支持 JSON/CSV 导入。

### 6.2 个人账号密码管理

- [x] 平台、账号、密码、URL 存储。
- [x] 密码 AES-256-GCM 加密存储。
- [x] 前端卡片式展示。
- [x] 密码按需解密并复制。
- [x] 按用户隔离查询。
- [x] Accounts 模块独立刷新，不触发 API Keys 刷新。
- [x] Accounts 列表样式优化：深色卡片、favicon、URL、密码复制、TOTP 与操作按钮分区展示。
- [x] Account 编辑能力安全增强：支持 platform / account / url / favicon_url 更新，更新密码或 TOTP Secret 时重新加密。
- [x] Accounts 支持按模块导出 JSON/CSV，并支持 JSON/CSV 导入。
- [x] 2FA/TOTP 动态验证码计算与显示，TOTP Secret 加密存储，按需生成 6 位动态码。
- [x] 网站图标自动抓取 Favicon，支持手动覆盖 favicon_url。

### 6.3 用户与权限

- [x] 首个注册用户自动成为 admin。
- [x] 后续用户需要邀请码注册。
- [x] 用户角色区分：admin / user。
- [x] Admin 专属菜单。
- [x] Admin 可创建邀请码。
- [x] Admin 可查看邀请码列表。
- [x] Admin 可按状态筛选邀请码并分页查看。
- [x] Admin 可删除未使用邀请码。
- [x] 普通用户禁止访问 Admin API。
- [x] 邀请码过期时间，默认 168 小时，Admin 创建时可调整过期小时数，注册时拒绝过期邀请码。
- [x] 邀请码使用人回填。
- [x] 登录防爆破策略增强：按 username + IP 统计失败次数，连续失败后 10 分钟冷却。
- [x] Master Key 复杂度校验增强。
- [x] 登录态升级为 AES-GCM sealed session token，避免 Master Key 出现在可读取 JWT claims 中。

### 6.4 审计日志

- [x] AuditLog 数据模型。
- [x] 批量导入 Key 写入审计日志。
- [x] 创建 Account 写入审计日志。
- [x] 审计日志列表展示。
- [x] 普通用户仅能查看自己的日志。
- [x] Admin 可查看全部日志。
- [x] Audit Logs 模块独立刷新。
- [x] 删除、更新、解密复制等动作补齐审计日志。
- [x] 审计日志分页。
- [x] 审计日志筛选。
- [x] 审计日志动作类型过滤。

### 6.5 可视化与交互

- [x] 深色系极简 Dashboard。
- [x] Sidebar 模块导航。
- [x] AI Keys 统计卡片。
- [x] API Key 按 Provider / Pool Group 双层分组。
- [x] Accounts 卡片展示。
- [x] Audit Logs 表格展示。
- [x] Admin Invitations 表格展示。
- [x] 空状态提示。
- [x] 搜索框。
- [x] 复制成功反馈。
- [x] 更精细的加载态与错误态。
- [x] 统一错误提示 Toast。
- [x] 登录页提供注册页面入口。
- [x] 更安全的剪贴板自动清理。
- [x] 移动端适配：Sidebar / Header / 卡片 / 表格区域支持小屏布局与横向滚动兜底。

### 6.6 容灾与数据迁移

- [x] 加密数据导出 JSON，并写入 `EXPORT_DATA_JSON` 审计日志。
- [x] 加密数据导出 CSV，并写入 `EXPORT_DATA_CSV` 审计日志。
- [x] 加密 JSON 导入，导入前校验密文字段为合法 AES-GCM ciphertext 形态，并限制单次导入条目数，避免把明文误导入敏感字段。
- [x] 离线解密脚本：`go run scripts/decrypt.go <export.json|ciphertext_base64> <master_key>`。
- [x] 数据库备份与恢复说明，见 README。


## 7. API 设计

### 7.1 Public API

| Method | Path | 说明 |
|---|---|---|
| POST | `/api/register` | 注册用户。首个用户自动 admin，后续用户需要邀请码。 |
| POST | `/api/login` | 登录并返回 sealed session token 与 role。 |

### 7.2 Protected API

所有接口都需要：

```http
Authorization: Bearer ***
```

#### API Keys

| Method | Path | 说明 |
|---|---|---|
| GET | `/api/keys/list` | 获取当前用户 API Key 列表，支持 `q` 搜索。 |
| GET | `/api/keys/stats` | 获取当前用户 Key 统计。 |
| POST | `/api/keys/create` | 按需新增一个或多个 API Key；每个条目必须提供 `key_name` 与 `key_value`，支持 `base_url` 与 `proxy_url`。 |
| POST | `/api/keys/bulk` | 兼容旧批量导入路径；新前端主入口使用 `/api/keys/create` 的多行表单。 |
| POST | `/api/keys/:id/check-quota` | 手动触发单个 API Key 健康探测；OpenAI-compatible / DeepSeek 使用 `/v1/models`，Anthropic 使用 `/v1/models`，Gemini 使用 `/v1beta/models`。未知 provider 只要配置了 `base_url`，按 OpenAI-compatible 中转站探测 `/v1/models`。当前写入可用性状态，不写入真实余额。 |
| PATCH | `/api/keys/:id` | 更新 API Key 元数据、`base_url`、`proxy_url` 或重新加密 key_value。 |
| DELETE | `/api/keys/:id` | 删除 API Key。 |
| GET | `/api/keys/:id/decrypt` | 按需解密指定 Key。 |

#### Accounts

| Method | Path | 说明 |
|---|---|---|
| GET | `/api/accounts/list` | 获取当前用户账号列表。 |
| POST | `/api/accounts/create` | 新增账号密码。 |
| PATCH | `/api/accounts/:id` | 更新账号，密码与 TOTP Secret 字段会重新加密。 |
| DELETE | `/api/accounts/:id` | 删除账号。 |
| GET | `/api/accounts/:id/decrypt` | 按需解密指定账号密码。 |
| GET | `/api/accounts/:id/totp` | 按需生成账号 TOTP 动态验证码。 |

#### Audit

| Method | Path | 说明 |
|---|---|---|
| GET | `/api/audit/list` | 获取审计日志。普通用户仅自己的日志，admin 全部可见。支持 `page`、`page_size`、`action`、`keyword`、`start_time`、`end_time` 筛选，返回 `items`、`total`、`page`、`page_size`、`total_pages`。 |
| GET | `/api/export/json` | 导出当前用户全部加密 JSON 数据。 |
| GET | `/api/export/csv` | 导出当前用户全部加密 CSV 数据。 |
| GET | `/api/export/keys/json` | 仅导出当前用户 API Keys 加密 JSON 数据。 |
| GET | `/api/export/keys/csv` | 仅导出当前用户 API Keys 加密 CSV 数据。 |
| GET | `/api/export/accounts/json` | 仅导出当前用户 Accounts 加密 JSON 数据。 |
| GET | `/api/export/accounts/csv` | 仅导出当前用户 Accounts 加密 CSV 数据。 |
| POST | `/api/import/json` | 导入全部加密 JSON 数据并归属到当前用户。 |
| POST | `/api/import/csv` | 导入全部加密 CSV 数据并归属到当前用户。 |
| POST | `/api/import/keys/json` | 仅导入 API Keys 加密 JSON 数据。 |
| POST | `/api/import/keys/csv` | 仅导入 API Keys 加密 CSV 数据。 |
| POST | `/api/import/accounts/json` | 仅导入 Accounts 加密 JSON 数据。 |
| POST | `/api/import/accounts/csv` | 仅导入 Accounts 加密 CSV 数据。 |

#### Admin

| Method | Path | 说明 |
|---|---|---|
| GET | `/api/admin/invites` | 获取邀请码列表，支持 `page`、`page_size`、`status=available|used|expired`，返回 `items`、`total`、`page`、`page_size`、`total_pages`。 |
| POST | `/api/admin/invites` | 创建邀请码，支持 `expires_in_hours`。 |
| DELETE | `/api/admin/invites/:id` | 删除未使用的邀请码；已使用邀请码不可删除。 |

### 7.3 路由规范

后端路由必须采用显式路径，避免以下问题：

- 静态路径如 `/keys/stats` 被动态路径 `/keys/:id` 抢占。
- 前端请求 `/api/audit`，后端只实现 `/api/audit/list` 导致 404。
- 列表接口与资源详情接口混用同一路径，后续扩展困难。

当前规范：

- 列表统一使用 `/list` 后缀。
- 创建特殊资源可使用语义化路径，如 `/bulk`、`/create`。
- 动态资源路径只用于明确的资源 ID 操作。

## 8. 前端数据加载规范

Dashboard 不允许使用一个全量 `fetchData` 同时刷新所有模块。

当前拆分：

- `fetchKeys()`：只拉取 API Key 列表。
- `fetchStats()`：只拉取 API Key 统计。
- `fetchAccounts()`：只拉取账号列表。
- `fetchLogs()`：只拉取审计日志，支持分页、动作类型过滤和关键词筛选。
- `fetchInvites()`：只拉取邀请码列表，支持分页与状态筛选。
- 敏感字段复制到剪贴板后，前端会在 30 秒后读取剪贴板；只有确认剪贴板内容仍等于本次复制的敏感值时才清空，避免误删用户后来复制的其他内容。
- `refreshActiveTab()`：根据当前 Tab 调度对应 Fetcher。

验收标准：

- 点击 API Keys 模块刷新，只允许请求 Keys 与 Stats。
- 点击 Accounts 模块刷新，只允许请求 Accounts。
- 点击 Audit Logs 模块刷新，只允许请求 Audit Logs。
- 点击 Admin 模块刷新，只允许请求 Invites。
- 切换 Tab 时只拉取目标模块数据。

## 9. 版本管理规范

项目采用 SemVer 风格的三段式版本号：

```text
MAJOR.MINOR.PATCH
```

当前初始版本：`0.0.0`。

### 9.1 版本号语义

- `MAJOR`：破坏性架构变化、数据模型不兼容、部署方式不兼容时递增。
- `MINOR`：新增向后兼容功能时递增。
- `PATCH`：Bug 修复、文档修正、样式优化、小型兼容性改动时递增。

### 9.2 0.x 阶段约定

项目在 `1.0.0` 前属于快速迭代期：

- `0.0.x`：早期原型与基础修复。
- `0.1.0`：第一个可公开体验版本，核心闭环可用。
- `0.2.0+`：逐步补齐安全增强、审计、导出、真实额度巡检等能力。
- `1.0.0`：零知识架构、安全边界、Docker 部署、核心 CRUD、审计与文档达到稳定可发布状态。

### 9.3 Git 标签规范

- Git Tag 使用 `vX.Y.Z` 格式，例如 `v0.1.0`。
- Release 标题使用同名版本号，例如 `v0.1.0`。
- 前端 `web/package.json` 中的 `version` 与项目版本保持一致。
- 后续如增加后端版本注入，应保证二进制、Docker 镜像、Git Tag 三者一致。

### 9.4 分支与提交规范

- 默认主分支：`master`。
- 首次推送前允许在 `master` 完成初始化提交。
- 首次推送后禁止直接在 `master` 分支修改代码。
- 后续所有变更必须从 `master` 创建工作分支。
- 功能开发：`feat/<description>`。
- Bug 修复：`fix/<description>`。
- 工程配置、文档、依赖、版本等维护类变更：`chore/<description>`。
- 分支命名遵循 `<type>/<description>` 格式，例如 `feat/xxx`、`fix/xxx`、`chore/xxx`。
- `type` 应体现变更性质，可按需要扩展，但必须保持斜杠分隔的规范格式。
- 提交信息采用 Conventional Commits：`feat:`、`fix:`、`chore:`、`refactor:`、`ci:`、`docs:`。

### 9.5 版本发布前检查

每次发布 Tag 前必须完成：

- 后端构建通过：`go build -o allinone-server ./cmd/server/main.go`。
- 前端构建通过：`bun run build`。
- `REQUIREMENTS.md` 与当前功能状态一致。
- 生产环境必须设置强随机 `ALLINONEKEY_JWT_SECRET` 与 `ALLINONEKEY_SESSION_SECRET`。
- `README.md` 的使用方式与部署方式无明显过期内容。
- 不包含明文密钥、数据库私密数据、构建产物或本地临时文件。

### 9.6 首次开源发布策略

- 当前暂不推送到 GitHub。
- 先完成并稳定基础功能，再推送第一版。
- 第一版推送前，`master` 可用于初始化提交。
- 第一版推送后，严格执行分支开发规范，不再直接修改 `master`。
- 当前 `web/package.json` 版本保持 `0.0.0`。
- 不创建 `v0.0.0` Release Tag。
- 第一个公开体验版本建议使用 `v0.1.0`。

## 10. 开发命令

### 10.1 后端

```bash
cd /home/allen/WorkSpace/Go/src/allinonekey
export GOPROXY=https://goproxy.cn,direct
/usr/local/go/bin/go mod tidy
ALLINONEKEY_DB_PATH=data/allinone.db ALLINONEKEY_JWT_SECRET=[REDACTED] ALLINONEKEY_SESSION_SECRET=[REDACTED] /usr/local/go/bin/go run ./cmd/server/main.go
```

本地默认数据库路径为 `data/allinone.db`。`make dev` 与 `make dev-server` 会自动创建 `data/` 目录，并通过 `ALLINONEKEY_DB_PATH` 指向该文件。

构建：

```bash
cd /home/allen/WorkSpace/Go/src/allinonekey
export GOPROXY=https://goproxy.cn,direct
go build -o allinone-server ./cmd/server/main.go
```

### 10.2 前端

```bash
cd /home/allen/WorkSpace/Go/src/allinonekey/web
bun install
bun dev
```

构建：

```bash
cd /home/allen/WorkSpace/Go/src/allinonekey/web
bun run build
```

### 10.3 Docker

```bash
cd /home/allen/WorkSpace/Go/src/allinonekey
ALLINONEKEY_JWT_SECRET=[REDACTED] ALLINONEKEY_SESSION_SECRET=[REDACTED] docker-compose up --build -d
# 或：
ALLINONEKEY_JWT_SECRET=[REDACTED] ALLINONEKEY_SESSION_SECRET=[REDACTED] make docker-up
```

停止旧容器：

```bash
cd /home/allen/WorkSpace/Go/src/allinonekey
docker-compose down
```

> 本地调试时必须确认旧 Docker 容器是否仍在占用 `8080`，否则浏览器可能一直访问容器内旧版本接口。

## 11. 项目结构

```text
allinonekey/
├── cmd/server/main.go              # 服务入口、Gin 路由、中间件
├── internal/api/                   # HTTP Handlers
│   ├── auth.go
│   ├── keys.go
│   ├── accounts.go
│   ├── admin.go
│   └── audit.go
├── internal/model/models.go         # GORM 模型
├── internal/service/quota.go        # 额度巡检服务
├── internal/util/                   # 加密、认证、工具函数
├── web/                             # Vue 3 前端
│   ├── src/views/Dashboard.vue
│   ├── src/api.ts
│   ├── src/store/
│   └── vite.config.ts
├── Dockerfile
├── docker-compose.yml
├── README.md
└── REQUIREMENTS.md
```

## 12. 当前进度记录

- [x] 初始化项目结构与 Go module。
- [x] 实现 AES-256-GCM 加解密工具。
- [x] 实现 Argon2id Master Key verifier。
- [x] 定义核心数据库模型。
- [x] 实现注册 / 登录。
- [x] 实现 JWT 鉴权中间件。
- [x] 实现 API Key 批量导入、列表、统计、解密、更新、删除。
- [x] 实现 Account 新增、列表、解密、更新、删除。
- [x] 实现 Admin 邀请码管理。
- [x] 实现 Audit Logs 列表。
- [x] 初始化 Vue 3 + Vite + TailwindCSS v4 + Pinia 前端。
- [x] 实现 Dashboard 暗色 UI。
- [x] 修复 `/api/keys/stats` 404。
- [x] 修复 `/api/audit` 404，统一为 `/api/audit/list`。
- [x] 修复已添加 Key 无法展示。
- [x] 修复 Account 列表无法展示。
- [x] 修复 Audit Logs 列表无法展示。
- [x] 修复 API Keys 刷新会同时刷新 Accounts 的不合理行为。
- [x] 编写 Docker multi-stage 构建。
- [x] 适配 Go 1.25.0 与 Bun 构建链路。
- [x] 实现 TOTP、Favicon、邀请码过期、登录防爆破、导出/导入与离线解密工具。

## 13. 已知问题与后续优化

### 13.1 高优先级

- [x] 后端 JWT Secret 改为读取 `ALLINONEKEY_JWT_SECRET` 环境变量。
- [x] Account Update 区分普通字段更新与密码更新，密码更新重新加密。
- [x] Key Update 实现真实字段更新。
- [x] 解密失败返回明确错误响应。
- [x] Delete / Decrypt / Update 操作补齐审计日志。
- [x] Docker Compose 与服务端统一 DB 路径为 `/app/data/allinone.db` / `data/allinone.db`。
- [x] Makefile 本地开发命令统一使用 `data/allinone.db`，并自动创建 `data/` 目录。
- [x] Docker 启动时通过 `PUID` / `PGID` 和 entrypoint 修正 `/app/data` 挂载目录权限，避免本地 `data/` 被 root:root 污染。
- [x] 本地开发测试统一使用 `8080` 端口；若被旧项目进程占用，可先释放该端口再启动。
- [x] Makefile 提供 `clean-data` / `reset-data`，用于忘记测试账号或 Master Key 后清理本地开发数据库。

### 13.2 中优先级

- [x] Audit Logs 增加分页、筛选、动作类型过滤。
- [x] API Keys 增加 provider/base_url/pool_group 编辑能力。
- [x] QuotaService 完成主流服务商 Key 健康探测：OpenAI-compatible / DeepSeek / Anthropic / Claude / Gemini。当前验证密钥可用性与网络状态，写入 `active` / `auth_error` / `rate_limited` / `quota_error` / `quota_unsupported`；不伪造真实余额。
- [x] 自定义 Provider / 第三方中转站：前端支持 Custom provider 与 Base URL，后端未知 provider + Base URL 时按 OpenAI-compatible 探测。
- [x] 统一错误提示 Toast。
- [x] 登录页提供注册页面入口。
- [x] 增加前端请求 Loading 状态。

### 13.3 低优先级

- [x] TOTP 支持。
- [x] Favicon 自动抓取。
- [x] 数据导出 / 导入。
- [x] 离线解密工具。
- [x] 移动端优化。

## 14. 验收标准

### 14.1 功能验收

- 注册首个用户后，该用户为 admin。
- 非首个用户必须使用有效邀请码注册。
- 登录后可进入 Dashboard。
- 批量导入 API Key 后，AI API Keys 列表立即展示新增数据。
- API Key 统计接口正常返回 total / active / error / balance。
- 单个 API Key 可手动触发健康探测；探测结果写入状态字段，真实余额未知时不得伪造 balance。
- 自定义 Provider / 第三方中转站可以通过自定义名称 + Base URL 导入，并按 OpenAI-compatible `/v1/models` 完成健康探测。
- API Key 可按 Provider / Pool Group 展示。
- 新增 Account 后，Accounts 列表立即展示新增数据。
- Audit Logs 页面可展示审计记录，并支持分页、动作类型过滤与关键词筛选。
- Admin 用户可以进入 Invitations 页面并创建邀请码。
- Admin 用户可以分页查看、按状态筛选邀请码，并删除未使用的邀请码。
- 邀请码过期后不能注册。
- 连续登录失败会触发冷却限制。
- Account 可保存加密 TOTP Secret，并按需生成动态码。
- 可导出加密 JSON / CSV，并可导入 JSON 备份。
- 普通用户不能访问 Admin API。

### 14.2 接口验收

- `GET /api/keys/stats` 不允许 404。
- `GET /api/keys/list` 不允许 404。
- `POST /api/keys/:id/check-quota` 不允许 404；无效 Key 应返回 `auth_error`，网络或 Provider 异常应返回 `quota_error` / `rate_limited`，未知 Provider 返回 `quota_unsupported`；配置 `proxy_url` 时只影响当前 Key 探测。
- `GET /api/accounts/list` 不允许 404。
- `GET /api/accounts/:id/totp` 对未配置 TOTP 的账号返回明确错误，对已配置 TOTP 的账号返回 6 位动态码。
- `GET /api/export/json` / `GET /api/export/csv` / `POST /api/import/json` 不允许 404。
- `GET /api/audit/list` 不允许 404，且分页 / 筛选返回结构包含 `items`、`total`、`page`、`page_size`、`total_pages`。
- 本地开发与自动验证默认使用 `8080` 端口；若端口被旧项目进程占用，允许先终止旧进程再验证当前版本。
- Docker 启动后宿主机 `data/` 目录和 `data/allinone.db` 应保持当前宿主用户可读写，不能被永久污染为 root-only。
- 忘记本地开发测试账号或 Master Key 时，可执行 `make clean-data` 删除本地 SQLite 数据库；若要清理后立即重新启动开发服务，可执行 `make reset-data`。
- 旧容器关闭后，本地 Vite 代理应正确转发到当前后端 `8080`。

### 14.3 前端刷新验收

- API Keys 刷新不触发 Accounts 请求。
- Accounts 刷新不触发 Keys 请求。
- Audit Logs 刷新不触发 Keys / Accounts 请求，分页与筛选也只触发 Audit Logs 请求。
- 搜索 API Keys 只触发 Keys 列表更新。

## 15. 工程边界

### Always

- 使用绝对路径操作项目文件。
- 修改前先读取相关文件。
- 首次推送后，修改代码前必须确认当前不在 `master`，并使用 `<type>/<description>` 格式的规范分支。
- 后端改动后执行 Go build。
- 前端改动后执行 `bun run build`。
- 路由新增必须同步更新本文档。
- 需求变更必须实时同步更新本文档。
- 涉及敏感数据的字段必须加密落库。

### Ask First

- 更换数据库。
- 引入 Redis、PostgreSQL、队列等新中间件。
- 改变零知识架构核心设计。
- 改变前端主技术栈。
- 修改部署拓扑。

### Never

- 明文落库 API Key 或密码。
- 把 Master Key 写入数据库。
- 在日志中打印 API Key、密码、Master Key。
- 前端列表接口直接返回敏感明文。
- 让动态路由覆盖静态业务路由。

## 16. 本阶段关键经验

- 404 问题曾被误判为后端路由问题，实际本地环境中旧 Docker 容器仍在响应请求；后续调试必须先确认端口占用和容器状态。
- Gin 路由仍应保持显式与无歧义，避免 `/keys/:id` 与 `/keys/stats` 这类潜在冲突。
- Dashboard 数据请求必须模块化，不应把所有模块绑在一个全量刷新函数上。
- 文档状态必须与真实代码保持一致，未实现能力不得标记为完成。
- Docker bind mount `./data:/app/data` 会由容器侧创建目录；为避免宿主机目录变成 `root:root`，镜像 entrypoint 必须先 `chown` 数据目录到 `PUID:PGID`，再用 `su-exec` 以宿主用户 ID 运行服务。
- 本地测试默认使用 `8080`，若旧的本项目进程占用端口，可以直接终止旧进程后再启动当前验证。
- 本地开发数据重置统一使用 `make clean-data`；它会先停止 Docker 容器并释放 `8080`，再删除 `data/allinone.db`、`data/allinone.db-shm`、`data/allinone.db-wal`。


