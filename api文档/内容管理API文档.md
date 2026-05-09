
# 内容管理 API 文档

## 概述

本文档描述内容管理相关的 API 接口，包括内容的上传、更新、删除、查询等操作。

---

## 接口列表

### 1. 上传内容

**接口地址**：`POST /api/content/upload`

**描述**：上传新内容（视频、图片、文字）

**认证**：需要登录 + 未被封禁

**请求方式**：`multipart/form-data`

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| title | string | 是 | 内容标题（1-200字符） |
| type | string | 是 | 内容类型：video、image、text |
| content | string | 否 | 文字内容（仅type=text时有效） |
| tags | string[] | 否 | 标签列表，支持多选 |
| file | file | 是 | 文件（仅type=video/image时有效） |

**响应示例**：

```json
{
  "code": 200,
  "message": "上传成功",
  "data": {
    "id": 1,
    "title": "测试内容",
    "type": "text",
    "tags": ["动画", "推荐"],
    "audit_status": "pending",
    "created_at": "2026-05-07T14:00:00Z"
  }
}
```

---

### 2. 更新内容

**接口地址**：`PUT /api/content/:id`

**描述**：更新已上传的内容

**认证**：需要登录 + 未被封禁，且只能更新自己的内容（管理员可更新任意内容）

**请求方式**：`multipart/form-data`

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| title | string | 否 | 新标题 |
| content | string | 否 | 新内容（仅文字类型） |
| tags | string[] | 否 | 新标签列表 |
| file | file | 否 | 新文件（仅视频/图片类型） |

**响应示例**：

```json
{
  "code": 200,
  "message": "更新成功",
  "data": {
    "id": 1,
    "title": "更新后的标题",
    "tags": ["动画", "新番"],
    "updated_at": "2026-05-07T15:00:00Z"
  }
}
```

**权限说明**：
- 普通用户只能更新自己的内容
- 管理员可以更新任意用户的内容
- 更新后的内容会重新进入审核状态

---

### 3. 删除内容

**接口地址**：`DELETE /api/content/:id`

**描述**：删除指定内容

**认证**：需要登录 + 未被封禁，且只能删除自己的内容（管理员可删除任意内容）

**请求参数**：无

**响应示例**：

```json
{
  "code": 200,
  "message": "删除成功",
  "data": null
}
```

**权限说明**：
- 普通用户只能删除自己的内容
- 管理员可以删除任意用户的内容
- 删除操作会同时删除相关的文件（图片、视频、封面图）

---

### 4. 获取内容详情

**接口地址**：`GET /api/content/:id`

**描述**：获取单个内容的详细信息

**认证**：无需登录

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | uint | 是 | 内容ID（URL路径参数） |

**响应示例**：

```json
{
  "code": 200,
  "message": "获取成功",
  "data": {
    "id": 1,
    "title": "测试内容",
    "type": "image",
    "content": "",
    "file_path": "1_1715065200000000000.jpg",
    "file_size": 102400,
    "user_id": 1,
    "tags": ["动画", "推荐"],
    "audit_status": "approved",
    "created_at": "2026-05-07T14:00:00Z",
    "updated_at": "2026-05-07T14:00:00Z",
    "User": {
      "ID": 1,
      "Username": "user1",
      "IsAdmin": false
    }
  }
}
```

---

### 5. 获取我的内容列表

**接口地址**：`GET /api/content/my`

**描述**：获取当前登录用户的内容列表

**认证**：需要登录

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，默认1 |
| page_size | int | 否 | 每页数量，默认20，最大100 |
| audit_status | string | 否 | 审核状态筛选：all、pending、approved、rejected |

**响应示例**：

```json
{
  "code": 200,
  "message": "获取成功",
  "data": {
    "list": [
      {
        "id": 1,
        "title": "我的内容",
        "type": "text",
        "tags": ["测试"],
        "audit_status": "approved"
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 20,
    "total_page": 1
  }
}
```

---

### 6. 获取内容列表

**接口地址**：`GET /api/content/list`

**描述**：获取公开的已审核内容列表

**认证**：无需登录

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，默认1 |
| page_size | int | 否 | 每页数量，默认20，最大100 |
| type | string | 否 | 内容类型筛选：video、image、text |
| tag | string | 否 | 按标签筛选 |
| keyword | string | 否 | 关键词搜索（标题或内容） |
| sort_by | string | 否 | 排序字段：created_at、updated_at、id |
| order | string | 否 | 排序方向：asc、desc |

**响应示例**：

```json
{
  "code": 200,
  "message": "获取成功",
  "data": {
    "list": [
      {
        "id": 1,
        "title": "公开内容",
        "type": "image",
        "tags": ["动画"],
        "image": "http://localhost:8080/uploads/xxx.jpg",
        "User": {
          "ID": 1,
          "Username": "user1"
        }
      }
    ],
    "total": 100,
    "page": 1,
    "page_size": 20,
    "total_page": 5
  }
}
```

---

### 7. 搜索内容

**接口地址**：`GET /api/content/search`

**描述**：搜索已审核通过的内容，支持关键词模糊匹配标题和内容

**认证**：无需登录

**请求参数**：

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| keyword | string | **是** | - | 搜索关键词，不能为空 |
| page | int | 否 | 1 | 页码，最小为1 |
| page_size | int | 否 | 20 | 每页数量，范围1-100 |

**搜索逻辑**：

| 规则 | 说明 |
|------|------|
| 审核过滤 | 仅搜索 `audit_status = approved`（已审核通过）的内容 |
| 模糊匹配 | 同时匹配 `title` 和 `content` 字段（使用 `LIKE '%keyword%'`） |
| 排序规则 | 按 `created_at DESC`（最新发布优先） |
| 关联数据 | 预加载 `User`、`BigTag`、`SmallTag` |

**响应示例**：

```json
{
  "code": 200,
  "message": "搜索成功",
  "data": {
    "list": [
      {
        "id": 1,
        "title": "测试内容标题",
        "type": "image",
        "content": "内容描述...",
        "user_id": 1,
        "tags": ["动画", "推荐"],
        "audit_status": "approved",
        "created_at": "2026-05-08T14:00:00Z",
        "User": {
          "ID": 1,
          "Username": "user1"
        }
      }
    ],
    "total": 10,
    "page": 1,
    "page_size": 20,
    "total_page": 1
  }
}
```

**错误响应**：

```json
{
  "code": 400,
  "message": "请输入搜索关键词",
  "data": null
}
```

---

### 8. 随机推荐

**接口地址**：`GET /api/content/recommend`

**描述**：获取随机推荐的内容

**认证**：无需登录

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| count | int | 是 | 返回数量（1-50） |
| tag | string | 否 | 按标签筛选 |

**响应示例**：

```json
{
  "code": 200,
  "message": "获取推荐成功",
  "data": {
    "list": [...],
    "count": 10
  }
}
```

---

### 9. 上传文章图片

**接口地址**：`POST /api/content/upload-image`

**描述**：上传文章中使用的图片

**认证**：需要登录 + 未被封禁

**请求方式**：`multipart/form-data`

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file | file | 是 | 图片文件（支持jpg、jpeg、png、gif、webp） |

**响应示例**：

```json
{
  "code": 200,
  "message": "上传成功",
  "data": {
    "id": 0,
    "filename": "1_1715065200000000000.jpg",
    "file_size": 102400,
    "image_url": "http://localhost:8080/uploads/1_1715065200000000000.jpg",
    "upload_time": "2026-05-07T14:00:00Z"
  }
}
```

---

## 错误响应

### 401 未登录

```json
{
  "code": 401,
  "message": "未登录",
  "data": null
}
```

### 403 无权限

```json
{
  "code": 403,
  "message": "无权修改此内容",
  "data": null
}
```

### 404 内容不存在

```json
{
  "code": 404,
  "message": "内容不存在",
  "data": null
}
```

### 400 请求参数错误

```json
{
  "code": 400,
  "message": "标题无效（1-200字符）",
  "data": null
}
```

---

## 权限说明

| 操作 | 普通用户 | 管理员 |
|------|----------|--------|
| 上传内容 | ✓（未封禁） | ✓ |
| 更新内容 | ✓（仅自己的） | ✓（任意） |
| 删除内容 | ✓（仅自己的） | ✓（任意） |
| 查看内容 | ✓（已审核） | ✓（全部） |

---

## 审核状态说明

| 状态 | 说明 |
|------|------|
| pending | 待审核 |
| approved | 已通过 |
| rejected | 已拒绝 |
