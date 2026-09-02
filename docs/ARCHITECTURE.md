# Chirp 后端架构与模块说明

本文档概述当前 Chirp 后端的整体架构、关键模块与运行配置，便于新同学快速上手与排障。

## 总览
- 语言/框架：Go 1.25+，Gorilla Mux。
- 架构风格：分层（Domain/Service/Repository/Handler），依赖倒置。
- 运行模式：**纯本地运行**。SQLite 单文件数据库 + 本地文件存储，无任何云服务依赖。
- SQLite 驱动：`modernc.org/sqlite`（纯 Go 实现，**无需 GCC/CGO**，Windows 开箱即用）。
- 配置来源：`config.json`（默认） + 环境变量覆盖，优先级：环境变量 > config.json > 默认值。

## 目录结构（关键部分）
```
cmd/server/main.go      # 入口与依赖注入
internal/config        # 配置加载
internal/domain        # 领域模型与仓库接口
internal/service       # 业务逻辑（Auth/Resource/Folder/Trash + Storage，含单元测试）
internal/repository    # 数据访问实现（sqlite）
internal/handler/http  # HTTP 路由与中间件
pkg/logger             # 日志初始化（stdout+logs/）
pkg/util               # 密码哈希（bcrypt）
docs/                  # 文档
scripts/               # 启动/测试脚本
uploads/               # 本地文件存储目录
logs/                  # 运行日志（已 .gitignore）
```

## 配置
默认配置文件：`config.json`（可用 `CONFIG_FILE` 指定路径）。全部字段：

| 字段 | 环境变量 | 默认值 | 说明 |
| :--- | :--- | :--- | :--- |
| `port` | `PORT` | `9527` | HTTP 监听端口 |
| `sqlitePath` | `SQLITE_PATH` | `chirp.db` | SQLite 数据库文件路径 |
| `jwtSecret` | `JWT_SECRET` | `default_secret` | JWT 签名密钥（生产务必修改） |
| `uploadDir` | `UPLOAD_DIR` | `uploads` | 上传文件存储目录 |

环境变量可覆盖同名字段，便于生产注入敏感信息（如 JWT 密钥）。

## 各层职责
- **Domain (`internal/domain`)**：按实体拆分：`user.go`（User+配额）、`resource.go`（文件元数据）、`folder.go`（文件夹）、`notification.go`（预留），各含对应仓库接口。无外部依赖。
- **Repository (`internal/repository/sqlite`)**：SQLite 实现，建表与幂等迁移在 `sqlite/db.go`（`users/resources/folders/notifications`）。
- **Service (`internal/service`)**：
  - `auth_service.go`：邮箱注册/登录、JWT 签发、配额查询。
  - `resource_service.go`：文件上传/下载/重命名/移动（含配额校验与记账）。
  - `folder_service.go`：纯文件夹 CRUD（创建/重命名/移动防环）。
  - `trash_service.go`：回收站编排（级联软删除/还原/彻底删除、物理清理、配额回收）。
  - `storage.go`：本地文件系统存储实现（`FileStorage` 接口）。
- **Handler (`internal/handler/http`)**：
  - 控制器按域拆分：`user_handler.go`、`resource_handler.go`、`drive_handler.go`（装配+错误映射）、`drive_folder_handler.go`、`drive_file_handler.go`、`drive_trash_handler.go`。
  - 中间件：认证/可选认证/管理员校验，`LoggingMiddleware`（请求日志）、`RecoverMiddleware`（panic 捕获）。
- **Pkg**：
  - `pkg/logger`：日志输出到 stdout+`logs/server-YYYYMMDD-HHMMSS.log`。
  - `pkg/util`：bcrypt 密码哈希。

## 运行与脚本
- 启动：`./scripts/run_server.sh`（默认使用 `config.json`，可设 `CONFIG_FILE`）。
- 单元测试：`go test ./internal/service/ -v`（18 个用例，见下节）。
- API 冒烟：`./scripts/test_api.sh`：MVP 基础流程（注册/登录/匿名上传/列表），需服务已启动。

## 单元测试
`internal/service/` 下四个测试文件，使用内存 fake 实现 Domain 仓库接口与 `FileStorage`（`testutil_test.go`），零外部依赖、可离线运行：

| 测试文件 | 覆盖点 |
| :--- | :--- |
| `folder_service_test.go` | 建夹/重命名/列表权限；移动防环（自身、直接子级、深层后代） |
| `resource_service_test.go` | 配额三连：超限拒绝（不落盘不记账）、恰好占满边界、上传成功记账；文件重命名/移动/删除的越权拦截 |
| `trash_service_test.go` | 级联软删、回收站顶层项过滤、子树还原、孤儿文件/文件夹重挂根目录、彻底删除（物理清理 + 配额精确回收） |

## 存储
- 文件写入 `uploadDir`，数据库记录 SHA256 哈希用于查重。
- 对外 URL 为相对路径 `/uploads/<filename>`，由内置静态文件服务提供（生产可用 Nginx 反代）。

## 日志
- 位置：`logs/server-YYYYMMDD-HHMMSS.log`（已加入 .gitignore），同时输出到 stdout。
- 中间件：记录 method/path/status/耗时；panic 记录 stack。

## 已知预留/未启用
- `Notification` 模型与表已建，但当前未在业务中使用（可后续扩展站内通知）。
- User 扩展字段（phone_number/school/student_id 等）保留在模型中，仅 school/student_id 等在资料更新接口使用。

## 常见排障
- **登录/认证失败**：确认 `JWT_SECRET` 一致；Header 为 `Authorization: Bearer <token>`。
- **数据库锁定**：SQLite 为单写库，避免多实例同时写同一个 `chirp.db`。

## 安全与提交
- `config.json` 已在 `.gitignore`，不要提交真实密钥。
- 生产环境使用环境变量覆盖敏感配置（JWT_SECRET）。

## 已移除的云依赖（历史说明）
当前版本为纯本地部署裁剪版，以下功能已被移除：
- 手机号短信验证码注册/登录（`/auth/send-code`、`/signup/phone`、`/login/phone`）。
- 阿里云短信通道（`pkg/sms`）与限流器（`pkg/limiter`）。
- 阿里云 OSS 对象存储（仅保留本地文件存储）。
- MySQL 支持（仅保留 SQLite）。
