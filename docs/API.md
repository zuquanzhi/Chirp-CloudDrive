# API 文档 (API Documentation)

本文档详细描述了 Chirp 后端服务提供的 RESTful API 接口。

## 1. 用户认证 (Authentication)

### 1.1 邮箱注册
*   **URL**: `/signup`
*   **Method**: `POST`
*   **Body**:
    ```json
    {
        "name": "User Name",
        "email": "user@example.com",
        "password": "password123"
    }
    ```
*   **Response**:
    ```json
    {
        "id": 1,
        "email": "user@example.com"
    }
    ```

### 1.2 邮箱登录
*   **URL**: `/login`
*   **Method**: `POST`
*   **Body**:
    ```json
    {
        "email": "user@example.com",
        "password": "password123"
    }
    ```
*   **Response**:
    ```json
    {
        "token": "eyJhbGciOiJIUzI1Ni..."
    }
    ```

### 1.3 获取当前用户信息
*   **URL**: `/api/me`
*   **Method**: `GET`
*   **Headers**: `Authorization: Bearer <token>`
*   **Response**:
    ```json
    {
        "id": 1,
        "name": "User Name",
        "email": "user@example.com",
        "phone_number": "11234567890",
        "created_at": "2023-01-01T00:00:00Z"
    }
    ```

### 1.4 更新当前用户信息
*   **URL**: `/api/me`
*   **Method**: `PATCH`
*   **Headers**: `Authorization: Bearer <token>`
*   **Body** (可选字段，留空则可置空该值):
    ```json
    {
        "name": "New Name",
        "school": "Test School",
        "student_id": "SID123",
        "birthdate": "2000-01-01",
        "address": "Test Address",
        "gender": "OTHER"
    }
    ```
*   **Response**: 更新后的用户对象
    ```json
    {
        "id": 1,
        "name": "New Name",
        "email": "user@example.com",
        "phone_number": "11234567890",
        "school": "Test School",
        "student_id": "SID123",
        "birthdate": "2000-01-01",
        "address": "Test Address",
        "gender": "OTHER",
        "created_at": "2023-01-01T00:00:00Z"
    }
    ```

## 2. 资源管理 (Resources)

### 2.1 上传资源
*   **URL**: `/api/public/resources`
*   **Method**: `POST`
*   **Headers**: 
    *   `Content-Type: multipart/form-data`
    *   `Authorization: Bearer <token>` (可选 - 若提供则关联上传者)
*   **Body (Form Data)**:
    *   `file`: (File) 文件对象
    *   `title`: (Text) 资源标题
    *   `description`: (Text) 资源描述
    *   `subject`: (Text) 学科/科目
    *   `type`: (Text) 资源类型 (如 "试卷", "笔记")
*   **Response**:
    ```json
    {
        "id": 1,
        "title": "Lecture Notes",
        "status": "PENDING",
        "file_hash": "...",
        "owner_id": 123, // 若已登录
        "url": "/uploads/uuid.ext"
    }
    ```

### 2.2 资源列表/搜索
*   **URL**: `/api/public/resources`
*   **Method**: `GET`
*   **Query Params**:
    *   `q`: 搜索关键词 (可选)
*   **Response**:
    ```json
    [
        {
            "id": 1,
            "title": "Lecture Notes",
            "description": "...",
            "created_at": "...",
            "url": "/uploads/uuid.ext"
        }
    ]
    ```

### 2.3 下载资源
*   **URL**: `/api/public/resources/{id}/download`
    *   注意: `{id}` 为资源 ID 数字，例如 `/api/public/resources/1/download`
*   **Method**: `GET`
*   **Response**: 文件流 (Binary Stream)

## 2.5 网盘接口 (Drive) — 均需登录

### 2.5.1 查询存储配额
*   **URL**: `/api/drive/quota`
*   **Method**: `GET`
*   **Headers**: `Authorization: Bearer <token>`
*   **Response**:
    ```json
    {
        "quota": 1073741824,
        "used": 0
    }
    ```

### 2.5.2 列出文件夹
*   **URL**: `/api/drive/folders`
*   **Method**: `GET`
*   **Headers**: `Authorization: Bearer <token>`
*   **Query Params**:
    *   `parent_id`: 父文件夹 ID（可选，缺省表示根目录）
*   **Response**:
    ```json
    [
        {
            "id": 1,
            "owner_id": 1,
            "parent_id": null,
            "name": "课件",
            "created_at": "2026-09-02T21:05:14Z"
        }
    ]
    ```

### 2.5.3 创建文件夹
*   **URL**: `/api/drive/folders`
*   **Method**: `POST`
*   **Headers**: `Authorization: Bearer <token>`
*   **Body**:
    ```json
    {
        "name": "课件",
        "parent_id": null
    }
    ```
*   **Response**: `201 Created`，返回文件夹对象

### 2.5.4 重命名 / 移动文件夹
*   **URL**: `/api/drive/folders/{id}`
*   **Method**: `PATCH`
*   **Headers**: `Authorization: Bearer <token>`
*   **Body**（两个字段可单独或同时提供）:
    ```json
    {
        "name": "新名称",
        "parent_id": 3
    }
    ```
    *   `name`: 重命名
    *   `parent_id`: 移动到目标文件夹；`null` 表示移回根目录
    *   移动到自身或其子孙文件夹会返回 `400`
*   **Response**: `{"message": "updated"}`

### 2.5.5 删除文件夹
*   **URL**: `/api/drive/folders/{id}`
*   **Method**: `DELETE`
*   **Headers**: `Authorization: Bearer <token>`
*   **说明**: 软删除（连同所有子孙文件夹及内部文件一并进入回收站）
*   **Response**: `204 No Content`

### 2.5.6 目录内容列表
*   **URL**: `/api/drive/items`
*   **Method**: `GET`
*   **Query Params**:
    *   `folder_id`: 文件夹 ID（可选，缺省表示根目录）
    *   `q`: 文件名搜索关键词（可选）
*   **Response**:
    ```json
    {
        "folders": [ { "id": 1, "name": "课件", "parent_id": null, "...": "..." } ],
        "files": [ { "id": 1, "original_name": "笔记.pdf", "size": 1024, "url": "/uploads/uuid.pdf", "...": "..." } ]
    }
    ```

### 2.5.7 上传文件
*   **URL**: `/api/drive/files`
*   **Method**: `POST`
*   **Headers**: `Content-Type: multipart/form-data`
*   **Body (Form Data)**:
    *   `file`: (File) 文件对象
    *   `folder_id`: (Text, 可选) 目标文件夹 ID，缺省为根目录
*   **说明**: 上传前校验配额，超限返回 `413 quota exceeded`；成功后计入 `used`
*   **Response**: `201 Created`，返回文件对象（个人网盘文件默认 `APPROVED`）

### 2.5.8 下载文件
*   **URL**: `/api/drive/files/{id}/download`
*   **Method**: `GET`
*   **Response**: 文件流（仅文件所有者可用，回收站中的文件不可下载）

### 2.5.9 重命名 / 移动文件
*   **URL**: `/api/drive/files/{id}`
*   **Method**: `PATCH`
*   **Body**（两个字段可单独或同时提供）:
    ```json
    {
        "name": "新文件名.pdf",
        "folder_id": 3
    }
    ```
    *   `name`: 重命名
    *   `folder_id`: 移动到目标文件夹；`null` 表示移回根目录

### 2.5.10 删除文件（移入回收站）
*   **URL**: `/api/drive/files/{id}`
*   **Method**: `DELETE`
*   **Response**: `204 No Content`

### 2.5.11 回收站列表
*   **URL**: `/api/drive/trash`
*   **Method**: `GET`
*   **说明**: 返回顶层已删除的文件夹与文件（位于已删除文件夹内的文件不单独列出）
*   **Response**: 结构同 `items`：`{"folders": [...], "files": [...]}`

### 2.5.12 还原回收站内容
*   **URL**: `/api/drive/trash/{kind}/{id}/restore`
*   **Method**: `POST`
*   **Path Params**: `kind` = `folders` | `files`
*   **说明**: 文件夹还原会级联还原其子树；原父目录已删除时自动挂回根目录

### 2.5.13 彻底删除
*   **URL**: `/api/drive/trash/{kind}/{id}`
*   **Method**: `DELETE`
*   **Path Params**: `kind` = `folders` | `files`
*   **说明**: 永久删除（物理文件一并删除、释放配额），**不可恢复**
*   **Response**: `204 No Content`

## 3. 管理员接口 (Admin)

### 3.1 审核资源
*   **URL**: `/api/admin/resources/{id}/review`
*   **Method**: `POST`
*   **Headers**: `Authorization: Bearer <token>`
*   **Body**:
    ```json
    {
        "status": "APPROVED" // 或 "REJECTED"
    }
    ```
*   **Response**: `200 OK`

### 3.2 查重检测
*   **URL**: `/api/admin/resources/duplicates`
*   **Method**: `GET`
*   **Headers**: `Authorization: Bearer <token>`
*   **Query Params**:
    *   `hash`: 文件哈希值
*   **Response**:
    ```json
    [
        {
            "id": 1,
            "title": "Existing File",
            "file_hash": "..."
        }
    ]
    ```

## 接口概览

### 公共接口 (Public)

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :---: |
| **POST** | `/signup` | 用户注册 | No |
| **POST** | `/login` | 用户登录 (返回 JWT) | No |
| **POST** | `/api/public/resources` | 资源上传 (支持匿名/多文件) | Optional |
| **GET** | `/api/public/resources` | 资源列表/搜索 (`?q=keyword`) | No |
| **GET** | `/api/public/resources/{id}/download` | 下载资源文件 | No |

### 用户接口 (User)

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :---: |
| **GET** | `/api/me` | 获取当前用户信息 | Yes |
| **PATCH** | `/api/me` | 更新当前用户资料 | Yes |

### 网盘接口 (Drive)

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :---: |
| **GET** | `/api/drive/quota` | 查询配额与已用空间 | Yes |
| **GET** | `/api/drive/items` | 目录内容列表 (`?folder_id=&q=`) | Yes |
| **GET** | `/api/drive/folders` | 列出文件夹 (`?parent_id=`) | Yes |
| **POST** | `/api/drive/folders` | 创建文件夹 | Yes |
| **PATCH** | `/api/drive/folders/{id}` | 重命名 / 移动文件夹 | Yes |
| **DELETE** | `/api/drive/folders/{id}` | 删除文件夹（软删除，级联子孙） | Yes |
| **POST** | `/api/drive/files` | 上传文件（multipart，校验配额） | Yes |
| **GET** | `/api/drive/files/{id}/download` | 下载文件 | Yes |
| **PATCH** | `/api/drive/files/{id}` | 重命名 / 移动文件 | Yes |
| **DELETE** | `/api/drive/files/{id}` | 文件移入回收站 | Yes |
| **GET** | `/api/drive/trash` | 回收站列表 | Yes |
| **POST** | `/api/drive/trash/{kind}/{id}/restore` | 还原（kind: folders/files） | Yes |
| **DELETE** | `/api/drive/trash/{kind}/{id}` | 彻底删除（释放配额） | Yes |

### 管理员接口 (Admin)

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :---: |
| **POST** | `/api/admin/resources/{id}/review` | 资源审核 (`{"status":"APPROVED"}`) | Yes |
| **GET** | `/api/admin/resources/duplicates` | 文件查重 (`?hash=...`) | Yes |

*注：所有受保护接口需在 Header 中携带 `Authorization: Bearer <token>`*

---

## 7. 新增接口 (v2)

### 在线预览 (Inline Preview)

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :---: |
| **GET** | `/api/drive/files/{id}/download?inline=1` | 在线预览（`Content-Disposition: inline`，按扩展名/嗅探返回正确的 Content-Type） | Yes |
| **GET** | `/api/shares/{token}/download?inline=1` | 分享文件在线预览 | No |

### 秒传 / 去重 (Instant Upload)

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :---: |
| **POST** | `/api/drive/files/instant` | 秒传：`{"name","hash","size","folder_id"}`，hash 已存在时直接创建文件条目，无需传输内容 | Yes |

*普通 multipart 上传与分片合并同样会自动命中去重：相同 SHA-256 的文件共享同一物理对象，物理删除按引用计数处理。*

### 分片 / 断点续传 (Chunked Upload)

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :---: |
| **POST** | `/api/drive/uploads/init` | 初始化：`{"filename","size","folder_id","file_hash","chunk_size"}`；hash 命中时直接返回 `instant:true` | Yes |
| **GET** | `/api/drive/uploads/{id}` | 查询会话状态（含已上传分片序号 `uploaded_chunks`，用于续传） | Yes |
| **PUT** | `/api/drive/uploads/{id}/chunks/{index}` | 上传分片（原始字节流） | Yes |
| **POST** | `/api/drive/uploads/{id}/complete` | 合并分片：校验总大小与 SHA-256 后生成文件 | Yes |
| **DELETE** | `/api/drive/uploads/{id}` | 放弃上传（清理分片） | Yes |

### 版本历史 (Version History)

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :---: |
| **GET** | `/api/drive/files/{id}/versions` | 列出版本组全部版本（新版本在前） | Yes |
| **POST** | `/api/drive/files/{id}/versions/{vid}/restore` | 恢复指定版本（生成一个新最新版本，复用旧物理对象） | Yes |

*同一目录下同名上传会自动生成 `version+1` 并保留旧版本；目录列表只展示最新版；删除/还原/彻底删除按版本组级联。*

### 分享链接 (Share Links)

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :---: |
| **POST** | `/api/drive/shares` | 创建分享：`{"resource_id","password"?,"expire_days"?}`（`expire_days<=0` 为永久） | Yes |
| **GET** | `/api/drive/shares` | 我的分享列表（含文件名、下载次数、过期时间） | Yes |
| **DELETE** | `/api/drive/shares/{id}` | 取消分享 | Yes |
| **GET** | `/api/shares/{token}` | 查看分享信息（公开，`has_password` 标记，不泄露提取码） | No |
| **GET** | `/api/shares/{token}/download?password=` | 通过分享下载（公开，错误提取码返回 403，过期返回 410） | No |

### 批量操作 (Batch Operations)

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :---: |
| **POST** | `/api/drive/batch/delete` | 批量移入回收站：`{"file_ids":[],"folder_ids":[]}`（单项失败不影响整体） | Yes |
| **POST** | `/api/drive/batch/move` | 批量移动文件：`{"file_ids":[],"folder_id":null}` | Yes |
| **POST** | `/api/drive/batch/download` | 打包下载：返回 zip 流（重名文件自动加序号） | Yes |

### 操作日志 (Activity Log)

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :---: |
| **GET** | `/api/activities?limit=50` | 当前用户的操作动态（上传/秒传/分片/重命名/移动/删除/还原/分享/版本等） | Yes |

### 回收站自动清理 (Trash Auto-Cleanup)

无独立接口。服务启动时立即执行一次，之后每小时扫描一次：**删除时间超过 30 天**的文件与文件夹会被彻底删除，物理对象按引用计数清理并回收配额。保留期见 `service.TrashRetention`。
