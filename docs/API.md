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

### 管理员接口 (Admin)

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :---: |
| **POST** | `/api/admin/resources/{id}/review` | 资源审核 (`{"status":"APPROVED"}`) | Yes |
| **GET** | `/api/admin/resources/duplicates` | 文件查重 (`?hash=...`) | Yes |

*注：所有受保护接口需在 Header 中携带 `Authorization: Bearer <token>`*
