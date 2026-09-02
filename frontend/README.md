# Chirp CloudDrive · Frontend

网盘系统前端，基于 **React 19 + TypeScript + Vite + Tailwind CSS + shadcn/ui**。

## 开发

```bash
cd frontend
npm install
npm run dev        # http://localhost:3000
```

开发服务器已配置代理（见 `vite.config.ts`）：`/api`、`/signup`、`/login`、`/uploads` → `http://localhost:9527`，请先启动后端（仓库根目录 `go run ./cmd/server/main.go`）。

## 构建

```bash
npm run build      # 输出 dist/
```

## 页面

| 路由 | 页面 | 功能 |
| :--- | :--- | :--- |
| `/login` | 登录/注册 | 邮箱登录、注册（注册后自动登录），JWT 存 localStorage |
| `/` | 我的网盘 | 面包屑导航、文件夹新建/重命名/移动/删除、文件上传/下载/重命名/移动/删除、当前目录搜索 |
| `/trash` | 回收站 | 还原、彻底删除（二次确认） |
| `/profile` | 个人中心 | 资料编辑、配额用量展示 |

未登录访问受保护页面自动跳转 `/login`。

## 目录结构

```text
frontend/
├── src/
│   ├── lib/api.ts          # API 客户端（fetch 封装，自动携带 JWT）
│   ├── lib/format.ts       # 展示格式化（文件大小、时间）
│   ├── hooks/use-auth.tsx  # 认证上下文（登录态管理）
│   ├── components/
│   │   ├── AppLayout.tsx   # 整体布局（侧边导航 + 配额条）
│   │   └── ui/             # shadcn/ui 组件
│   ├── pages/              # 上述 4 个页面
│   ├── types/              # TypeScript 类型定义
│   ├── App.tsx             # 路由与守卫
│   └── main.tsx
└── vite.config.ts          # dev 代理配置
```

## 交互

- 拖拽上传：将文件拖入网盘主页列表区即可上传到当前目录
- 拖拽移动：拖动文件/文件夹行到目标文件夹行（或面包屑节点）完成移动

## 质量检查

```bash
npm run lint       # ESLint（components/ui 为 shadcn 生成代码，其提示可忽略）
npm run build      # 类型检查 + 产物构建
```

> 后端 API 契约见 `docs/API.md` 与 `docs/NETDISK_DESIGN.md`。
