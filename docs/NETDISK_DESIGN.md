# Chirp CloudDrive · 网盘系统课设设计方案

> 本文档是**设计稿**（写于实施前），保留作为设计过程的记录。最终交付与设计稿的偏差如下：
>
> - **已按设计实现**：存储配额（1GB）、文件夹层级管理、文件上传/下载/重命名/移动、回收站（软删除/还原/彻底删除）、个人中心。
> - **范围裁剪（明确不做）**：秒传（含 `ref_count` 引用计数）与文件分享（`shares` 表及公开路由）在课设范围控制中被砍掉，数据库未建 `shares` 表、`resources` 表无 `ref_count` 字段；彻底删除直接物理清理并回收配额。
> - **前端**：由"预留目录"落地为 React 19 + Vite + Tailwind + shadcn/ui 工程（`frontend/`），含拖拽上传与拖拽移动。
> - **结构演进**：`internal/domain/models.go` 拆分为按实体四文件；回收站逻辑独立为 `trash_service.go`；drive handler 按 folder/file/trash 拆为三文件。
>
> 以 `docs/API.md` 和 `docs/ARCHITECTURE.md` 为当前实现的准绳。

## 1. 项目定位

面向校园用户的轻量网盘：注册登录后拥有个人云空间，支持文件夹管理、文件上传下载、秒传、回收站、分享链接，管理员可审核与配额管理。

**现有代码可直接复用的能力：**

| 已有能力 | 位置 | 在网盘中的角色 |
| :--- | :--- | :--- |
| 邮箱注册/登录 + JWT | `internal/service/auth_service.go` | 用户系统，直接沿用 |
| 文件上传/下载/列表 | `internal/service/resource_service.go` | 文件核心流程的雏形，需扩展目录/重命名等 |
| SHA256 哈希查重 | `resource_service.go` 上传流程 | 升级为"秒传"功能的基础 |
| 管理员审核/查重接口 | `internal/handler/http/resource_handler.go` | 管理端保留 |
| 本地文件存储抽象 | `internal/service/storage.go` (`FileStorage` 接口) | 存储层不变 |
| 用户资料字段（school 等） | `internal/domain/models.go` | 个人中心沿用 |

## 2. 功能规划

### 2.1 MVP（课设必做）

1. **用户系统**：注册、登录、个人资料（已有，保留）；新增**存储配额**（每用户固定额度，如 1GB，显示已用/总量）。
2. **我的网盘**：
   - 文件夹：创建、重命名、删除、层级嵌套、面包屑导航
   - 文件：上传（multipart）、下载、重命名、移动到其他文件夹、删除
   - 列表：按名称/大小/时间排序，按名称搜索
3. **秒传**：上传前先传文件 SHA256，若服务端已有相同哈希文件则直接建立引用，跳过传输（现有哈希字段扩展为全局查重）。
4. **回收站**：删除为软删除，进入回收站；可还原、可彻底删除。
5. **文件分享**：生成分享链接 + 4 位提取码，可设有效期；访客凭链接+提取码下载。

### 2.2 进阶（加分项，按时间选做）

- 大文件**分片上传**与断点续传（chunk 上传 + 合并）
- 图片/文本/PDF **在线预览**
- 分享管理页（我创建的分享列表、取消分享）
- 操作日志（上传/下载/删除记录）
- 管理端：用户列表、配额调整、违规文件处理（沿用现有审核状态机 PENDING/APPROVED/REJECTED）

### 2.3 明确不做（控制课设范围）

- 多设备同步、WebDAV、协作编辑
- 分布式存储（保持本地单盘，架构上留 `FileStorage` 接口即可）

## 3. 整体架构

沿用现有 Clean Architecture 分层，仅做增量扩展：

```
┌────────────┐   HTTP/JSON    ┌─────────────────────────────────┐
│  frontend/ │ ◄────────────► │  Go Backend (repo root)         │
│  (预留)     │                │  handler → service → repository │
└────────────┘                │       ↓              ↓          │
                              │  uploads/ (文件)   chirp.db     │
                              └─────────────────────────────────┘
```

- **后端**：保持仓库根目录即 Go module（`github.com/zuquanzhi/Chirp/backend`），不改动现有分层。
- **前端**：预留 `frontend/` 目录（见第 6 节），独立前端工程，开发期通过 dev server 代理到 `http://localhost:9527` 联调。
- **存储**：数据库记录元数据，文件本体仍在 `uploads/`；秒传通过"多记录指向同一物理文件"实现，物理文件引用计数归零才真正删除。

## 4. 数据库设计（SQLite 演进）

在现有 `users` / `resources` / `notifications` 三表基础上演进。**改造而非推翻**：现有 `resources` 表演进为 `files` 表语义。

### 4.1 users（扩展）

| 新增字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| `quota` | INTEGER | 配额字节数，默认 1073741824 (1GB) |
| `used` | INTEGER | 已用字节数，上传/删除时维护 |

（现有字段：id, name, email, password, role, created_at, phone_number, school, student_id, birthdate, address, gender —— 全部保留）

### 4.2 files（由 resources 演进）

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| id | INTEGER PK | 现有 |
| owner_id | INTEGER | 现有，网盘中**必填**（网盘文件必须属于用户，取消匿名上传） |
| folder_id | INTEGER NULL | **新增**，所属文件夹，NULL 表示根目录 |
| title / description | TEXT | 现有 |
| filename | TEXT | 物理存储名（UUID），现有 |
| original_name | TEXT | 现有 |
| size | INTEGER | 现有 |
| file_hash | TEXT | 现有，秒传依据 |
| status | TEXT | 现有（PENDING/APPROVED/REJECTED），个人文件可默认 APPROVED |
| deleted_at | DATETIME NULL | **新增**，软删除标记（回收站） |
| ref_count | INTEGER | **新增**，物理文件引用计数（配合秒传） |
| created_at | DATETIME | 现有 |
| subject / type | TEXT | 现有，可保留作分类标签 |

### 4.3 folders（新增）

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| id | INTEGER PK | |
| owner_id | INTEGER | 所属用户 |
| parent_id | INTEGER NULL | 父文件夹，NULL 为根 |
| name | TEXT | 文件夹名 |
| created_at / deleted_at | DATETIME | 软删除同 files |

### 4.4 shares（新增）

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| id | INTEGER PK | |
| file_id | INTEGER | 分享的文件 |
| owner_id | INTEGER | 创建者 |
| code | TEXT | 4 位提取码 |
| token | TEXT UNIQUE | 分享链接随机串 |
| expires_at | DATETIME NULL | 过期时间，NULL 永久 |
| created_at | DATETIME | |

## 5. API 设计（增量）

已有保留：`POST /signup`、`POST /login`、`GET/PATCH /api/me`、`GET /api/admin/*`。

网盘新增（均需登录，统一前缀 `/api/drive`）：

| Method | Endpoint | 说明 |
| :--- | :--- | :--- |
| GET | `/api/drive/quota` | 查询配额与已用空间 |
| GET | `/api/drive/items?folder_id=&q=&sort=` | 列出某文件夹下的文件夹+文件（支持搜索/排序） |
| POST | `/api/drive/folders` | 创建文件夹 `{name, parent_id}` |
| PATCH | `/api/drive/folders/{id}` | 重命名/移动文件夹 |
| DELETE | `/api/drive/folders/{id}` | 删除（连同内容进回收站） |
| POST | `/api/drive/files/check` | **秒传检查** `{file_hash, size}` → 已存在则直接成功 |
| POST | `/api/drive/files` | 上传文件（multipart，含 folder_id） |
| GET | `/api/drive/files/{id}/download` | 下载 |
| PATCH | `/api/drive/files/{id}` | 重命名/移动 |
| DELETE | `/api/drive/files/{id}` | 移入回收站 |
| GET | `/api/drive/trash` | 回收站列表 |
| POST | `/api/drive/trash/{id}/restore` | 还原 |
| DELETE | `/api/drive/trash/{id}` | 彻底删除（ref_count 归零时删物理文件） |
| POST | `/api/drive/shares` | 创建分享 `{file_id, expire_hours}` → `{token, code}` |
| GET | `/api/drive/shares` | 我的分享列表 / 取消（DELETE `/{id}`） |
| POST | `/api/share/{token}/verify` | **公开**：校验提取码 |
| GET | `/api/share/{token}/download` | **公开**：提取码校验通过后下载 |

旧路由 `/api/public/resources*` 可在迁移期保留兼容，稳定后下线。

## 6. 目录结构（含前端预留）

```
/                          ← 仓库根 = Go 后端 module（现状不变）
├── cmd/server/main.go
├── internal/
│   ├── config/  domain/  service/  handler/http/
│   └── repository/sqlite/
├── pkg/logger/  pkg/util/
├── frontend/              ← 【本次预留】前端工程根目录
│   └── README.md          ← 占位说明：技术栈与规划，暂无代码
├── uploads/  logs/  docs/  scripts/
├── config.example.json
└── go.mod
```

**前端技术栈建议**（课设友好）：Vue 3 + Vite + Element Plus + Pinia + Axios；若更熟悉 React 则 React + Vite + Ant Design。开发联调：Vite `server.proxy` 将 `/api`、`/uploads` 代理到 `http://localhost:9527`。

前端页面清单（对应 2.1 的功能）：登录/注册页、网盘主页（文件列表+面包屑+上传）、回收站页、分享管理页、访客提取页、个人中心（配额展示）、（可选）管理端审核页。

## 7. 实施路线（建议 5 个里程碑）

| 阶段 | 内容 | 对现有代码的改动 |
| :--- | :--- | :--- |
| M1 | 配额 + 文件夹模型与接口 | domain 加 `Folder`、users 表加列、sqlite 迁移 |
| M2 | 文件归属文件夹、重命名/移动/软删除 | `resources`→`files` 演进、回收站逻辑 |
| M3 | 秒传 + ref_count | 上传流程改造，`/files/check` 新接口 |
| M4 | 分享链接 + 提取码 | domain 加 `Share`、公开路由 |
| M5 | 前端工程搭建并联调 | `frontend/` 内开发，纯增量 |

每个里程碑结束跑 `go build ./...` + `scripts/test_api.sh` 回归，保证课设任何时刻都有可演示版本。

## 8. 课设答辩可讲的点

- Clean Architecture 分层与依赖注入的实践
- 秒传原理（内容寻址 SHA256 + 引用计数）
- 软删除/回收站的事务一致性（删文件夹级联、还原冲突）
- JWT 无状态认证与中间件链
- SQLite 纯 Go 驱动（modernc.org/sqlite）零依赖部署
