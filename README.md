# Chirp (知了) - CloudDrive 网盘系统

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Chirp CloudDrive** 是一个面向大学生的网盘系统（课程设计）：Go 后端 + React 前端，支持文件夹管理、文件上传下载、回收站与存储配额。

当前版本为**纯本地部署版**：SQLite 单文件数据库 + 本地文件存储，零云服务依赖，开箱即用。

## 架构设计

本项目遵循 **Clean Architecture**（整洁架构）原则，结合 **Standard Go Project Layout** 进行组织。旨在实现高内聚、低耦合，确保业务逻辑独立于外部框架和驱动。

### 分层说明

*   **Domain Layer (`internal/domain`)**: 核心业务实体与接口定义，按实体拆分（`user.go` / `resource.go` / `folder.go` / `notification.go`）。不依赖任何外部库。
*   **Service Layer (`internal/service`)**: 具体的业务逻辑实现：`auth_service`（认证/配额）、`resource_service`（文件）、`folder_service`（文件夹）、`trash_service`（回收站编排）。依赖于 Domain 层接口。
*   **Repository Layer (`internal/repository`)**: 数据持久化适配器。实现了 Domain 层定义的 Repository 接口（SQLite）。
*   **Handler Layer (`internal/handler`)**: 接口适配层。负责处理 HTTP 请求，解析参数并调用 Service 层。
*   **Config (`internal/config`)**: 集中式配置管理。

### 目录结构

```text
/
├── cmd/
│   └── server/
│       └── main.go           # 应用程序入口，负责依赖注入与服务启动
├── internal/
│   ├── config/               # 配置加载与管理
│   ├── domain/               # 领域模型与仓库接口（user/resource/folder/notification 四文件）
│   ├── handler/              # HTTP 处理器 (REST API)
│   │   └── http/             # 具体 HTTP Handler 实现与中间件（drive 按 folder/file/trash 拆分）
│   ├── repository/           # 数据访问层实现
│   │   └── sqlite/           # SQLite 实现
│   └── service/              # 业务逻辑层（含 *_test.go 单元测试）
├── pkg/                      # 公共库 (可被外部项目复用)
│   └── util/                 # 工具函数 (如 Password Hashing)
├── frontend/                 # React 前端 (Vite + TS + Tailwind + shadcn/ui)
├── uploads/                  # 本地文件存储目录
├── go.mod                    # 依赖管理
└── README.md                 # 项目文档
```

## 快速开始 (Getting Started)

### 前置要求

*   **Go**: 1.25 或更高版本
*   **Node.js**: 18+（仅前端开发需要）
*   无需 GCC：SQLite 驱动使用纯 Go 实现的 `modernc.org/sqlite`，Windows 上开箱即用。

## API 文档 (API Documentation)

详细的 API 接口文档请参考 [docs/API.md](docs/API.md)。

### 接口概览

*   **用户认证**: 邮箱注册、邮箱登录、获取/更新用户信息
*   **网盘 (Drive)**: 文件夹管理、文件上传/下载/重命名/移动、目录搜索、存储配额
*   **回收站**: 还原、彻底删除（释放配额）
*   **管理员**: 资源审核、查重

## 本地开发环境搭建

1.  **克隆仓库**

    ```bash
    git clone https://github.com/zuquanzhi/Chirp-CloudDrive.git
    cd Chirp-CloudDrive
    ```

2.  **配置（可选）**

    程序启动时默认读取当前目录的 `config.json`，不存在时使用内置默认值。也可通过环境变量 `CONFIG_FILE` 指定路径。

    复制 `config.example.json` 为 `config.json` 并按需修改：

    ```json
    {
        "port": "9527",
        "sqlitePath": "chirp.db",
        "jwtSecret": "dev_secret_key",
        "uploadDir": "uploads"
    }
    ```

    环境变量可覆盖同名配置（优先级：环境变量 > config.json > 默认值）。

3.  **安装依赖**

    ```bash
    go mod tidy
    ```

4.  **启动服务**

    使用提供的脚本一键启动：
    ```bash
    ./scripts/run_server.sh
    ```
    或者手动运行：
    ```bash
    go run ./cmd/server/main.go
    ```

5.  **运行测试**

    Service 层单元测试（内存 fake 实现仓库/存储接口，零外部依赖）：

    ```bash
    go test ./internal/service/ -v
    ```

    覆盖：文件夹移动防环（自身/子级/后代）、配额记账（超限拒绝/恰好占满/上传记账）、回收站级联（软删/还原/孤儿重挂根目录/彻底删除配额回收）共 18 个用例。

    端到端 API 冒烟测试（需服务已启动）：

    ```bash
    ./scripts/test_api.sh
    ```

6.  **启动前端（另开一个终端）**

    ```bash
    cd frontend
    npm install
    npm run dev        # http://localhost:3000，已配置代理到后端 :9527
    ```

    详见 [frontend/README.md](frontend/README.md)。

## 技术栈

*   **Backend**: Go + Gorilla Mux + SQLite (`modernc.org/sqlite`，纯 Go 无 CGO) + JWT + bcrypt
*   **Frontend**: React 19 + TypeScript + Vite + Tailwind CSS + shadcn/ui
*   **Storage**: Local Filesystem

## 开发规范

*   **代码风格**: 遵循 `go fmt` 标准。
*   **错误处理**: 尽量在 Service 层处理业务错误，Handler 层处理 HTTP 状态码映射。
*   **依赖注入**: 严禁在业务逻辑中直接初始化 Repository，必须通过构造函数注入。

## 路线图 (Roadmap)

*   [x] 基础用户认证 (Signup/Login/JWT)
*   [x] 资源上传与下载 (Local Storage)
*   [x] 架构重构 (Clean Architecture)
*   [x] 纯本地化裁剪（SQLite + 本地存储，移除云服务依赖）
*   [x] 网盘 M1：存储配额 + 文件夹管理
*   [x] 网盘 M2：文件管理 + 回收站（还原/彻底删除/配额回收）
*   [x] React 前端（网盘主页/回收站/个人中心，拖拽上传与拖拽移动）
*   [x] Service 层单元测试（18 用例：移动防环/配额记账/回收站级联）
*   [ ] Docker 容器化部署支持

---
© 2025 Chirp Team. All Rights Reserved.
