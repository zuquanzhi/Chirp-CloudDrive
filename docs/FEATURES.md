# Chirp CloudDrive v2 新增功能

本次迭代在原有「配额 + 文件夹 + 回收站」基础上新增 8 项功能，全部保持 Clean Architecture 分层（domain 定义接口、service 实现业务、sqlite 适配持久化、http 适配路由），并配套单元测试与端到端冒烟脚本。

## 1. 文件在线预览

- 下载接口新增 `?inline=1` 参数：`Content-Disposition: inline`，并按扩展名（`mime.TypeByExtension`）或内容嗅探（`http.DetectContentType`）返回正确的 `Content-Type`。
- 前端点击文件名弹出预览对话框：图片 / PDF / 视频 / 音频 / 文本（含 Markdown 源码）直接渲染，其他类型提示下载。
- 分享链接同样支持 `inline=1` 预览。

## 2. 分享链接

- 新表 `shares`：随机 24 位十六进制 token、可选提取码、可选有效期（1/7/30 天或永久）、下载计数。
- 管理端（需登录）：创建 / 列表 / 取消分享；前端「我的分享」页面集中管理。
- 公开端（免登录）：`/api/shares/{token}` 查看信息（只返回 `has_password` 标记，不泄露提取码）、`/api/shares/{token}/download?password=` 下载；错误提取码 403、过期 410。
- 前端公开页 `/share/:token`，无需账号即可输入提取码下载。

## 3. 批量操作

- `POST /api/drive/batch/delete`：批量移入回收站（文件 + 文件夹混合，单项失败不影响整体，返回失败明细）。
- `POST /api/drive/batch/move`：批量移动文件。
- `POST /api/drive/batch/download`：流式 zip 打包下载（`archive/zip`，重名自动加序号）。
- 前端列表新增复选框与批量工具栏。

## 4. 回收站自动清理

- 服务启动时立即执行一次，之后每小时扫描：`deleted_at` 超过 **30 天**（`service.TrashRetention`）的文件与文件夹被彻底删除。
- 物理对象按引用计数清理，配额同步回收；清理数量写入服务日志。

## 5. 秒传 / 去重存储

- 所有上传路径（multipart / instant / 分片合并）统一经过 `saveDriveFile`：先按 SHA-256 查找已有内容，命中则**跳过写盘**，新条目直接指向同一物理对象。
- 物理删除改为**引用计数**：只有不再被任何条目引用时才真正删盘（`CountByFilename`）。
- 配额按逻辑文件计费（每个条目各计一次），保证记账与回收一致。
- 前端上传前先计算 WebCrypto SHA-256 并尝试 `POST /api/drive/files/instant`，命中即「秒传」。

## 6. 分片 / 断点续传

- 新表 `upload_sessions`：uuid 会话 + 文件名/大小/分片大小/总分片数/哈希。
- 流程：`init`（hash 命中直接秒传）→ `PUT chunks/{index}` 逐片上传（落在 `uploads/.chunks/<session>/`）→ `complete` 顺序合并、校验总大小与 SHA-256 后入库。
- 断点续传：`GET /api/drive/uploads/{id}` 返回磁盘上已存在的分片序号，客户端只补传缺失分片。
- 前端策略：≤ 8MB 走普通上传，> 8MB 自动分片（8MB/片），右下角实时进度面板。

## 7. 文件版本历史

- `resources` 表新增 `version_group` / `version` / `is_latest` 三列（幂等迁移，旧数据默认 v1 最新）。
- 同目录同名上传自动生成新版本：旧版 `is_latest=0`，新版 `version+1`；目录列表与回收站只展示最新版。
- `GET .../versions` 查看全部版本、`POST .../versions/{vid}/restore` 恢复旧版（生成新最新版，复用旧物理对象）。
- 软删 / 还原 / 彻底删除按**版本组级联**；旧版物理对象同样受引用计数保护。

## 8. 操作日志 / 最近动态

- 新表 `activities` + `ActivityRecorder`（**best-effort**：写日志失败不影响业务）。
- 覆盖动作：上传 / 秒传 / 分片上传 / 新版本 / 重命名 / 移动 / 删除 / 还原 / 彻底删除 / 创建与取消分享 / 恢复版本 / 文件夹增改移。
- `GET /api/activities?limit=` 查询，前端「最近动态」页面以时间线 + 图标展示。

---

## 测试

- **单元测试**（`go test ./internal/service/`）：25 个用例全绿，新增覆盖秒传去重、引用计数保物理文件、版本历史（生成/列表/恢复）、过期清理、分享生命周期（提取码/计数/取消/过期）。
- **端到端冒烟**（`scripts/test_new_features.sh`）：自建临时库启动真实服务，29 项断言全绿，覆盖注册登录、秒传去重（磁盘只有一个物理文件）、版本 v2/v3、inline 预览头、分享提取码 403/200、批量 zip、3 分片续传合并、动态流水。

## 前端页面一览

| 页面 | 路由 | 说明 |
| :--- | :--- | :--- |
| 我的网盘 | `/` | 复选框批量操作、点击文件名预览、行内分享/版本入口、智能上传进度面板 |
| 我的分享 | `/shares` | 全部分享链接的状态/下载次数/过期时间管理 |
| 最近动态 | `/activities` | 操作时间线 |
| 公开分享页 | `/share/:token` | 免登录提取码下载 |
| 回收站 | `/trash` | 还原 / 彻底删除（30 天后自动清理） |

接口明细见 [API.md](API.md) 第 7 章。
