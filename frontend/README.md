# Chirp CloudDrive · Frontend（预留目录）

本目录为网盘系统的**前端工程预留位置**，当前暂无代码。

## 技术栈（建议）

- Vue 3 + Vite + Element Plus + Pinia + Vue Router + Axios
- （备选）React + Vite + Ant Design

## 联调方式

- 后端服务运行在 `http://localhost:9527`（仓库根目录，`go run ./cmd/server/main.go`）
- 开发期在 Vite 配置 `server.proxy`，将 `/api`、`/uploads` 代理到后端

## 规划页面

对应 `docs/NETDISK_DESIGN.md` 第 2.1 节的功能范围：

1. 登录 / 注册
2. 网盘主页（文件夹树、文件列表、面包屑、上传、重命名、移动、删除）
3. 回收站（还原 / 彻底删除）
4. 分享管理（创建分享、提取码、取消）
5. 访客提取页（输入提取码下载）
6. 个人中心（配额用量展示）
7. （可选）管理端审核页

## 计划目录结构

```text
frontend/
├── index.html
├── package.json
├── vite.config.js
└── src/
    ├── api/        # Axios 封装，对接后端 REST API
    ├── assets/
    ├── components/ # 通用组件（文件列表、上传器等）
    ├── router/
    ├── stores/     # Pinia（用户态、当前目录态）
    ├── views/      # 上述页面
    └── main.js
```

> 具体 API 契约以后端 `docs/API.md` 与 `docs/NETDISK_DESIGN.md` 第 5 节为准。
