# 评论 API 文档

## 概述

本文档描述评论相关的 API 接口，包括添加评论、获取评论列表、删除评论、举报评论等操作，包含反垃圾和和谐功能。

---

## 反垃圾和和谐功能

### 功能特性

| 功能 | 说明 |
|------|------|
| 关键词过滤 | 自动检测评论内容中的敏感词（从外置JSON词库读取），包含不当内容的评论将被拒绝 |
| 内容长度限制 | 评论内容最大5000字符 |
| 异步反垃圾API | 评论发布后异步调用外置反垃圾API，返回code=966时自动封禁评论 |
| 评论举报 | 用户可以举报不当评论，管理员处理 |

### 封禁状态

| 状态 | 说明 |
|------|------|
| is_banned=false | 正常显示 |
| is_banned=true | 被封禁，不可见 |

---

## 接口列表

### 1. 添加评论

**接口地址**：`POST /api/comment/add`

**描述**：为指定内容添加评论，支持回复评论（二级评论）。评论发布后直接展示，同时异步调用反垃圾API进行二次检查。

**认证**：需要登录 + 未被封禁

**请求方式**：`application/x-www-form-urlencoded`

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| content_id | uint | 是 | 内容ID |
| text | string | 是 | 评论内容（最大5000字符） |
| parent_id | uint | 否 | 父评论ID（回复评论时使用） |

**响应示例**：

```json
{
  "code": 200,
  "message": "评论成功",
  "data": {
    "id": 1,
    "content_id": 1,
    "user_id": 1,
    "text": "这是一条评论",
    "parent_id": null,
    "is_banned": false,
    "created_at": "2026-05-08T14:00:00Z"
  }
}
```

**错误响应**：

```json
{
  "code": 400,
  "message": "内容包含违规词汇",
  "data": null
}
```

---

### 2. 获取评论列表

**接口地址**：`GET /api/comment/list/:content_id`

**描述**：获取指定内容的未封禁评论列表（包含回复）

**认证**：无需登录

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| content_id | uint | 是 | 内容ID（URL路径参数） |

**响应示例**：

```json
{
  "code": 200,
  "message": "获取成功",
  "data": [
    {
      "id": 1,
      "content_id": 1,
      "user_id": 1,
      "text": "这是一条评论",
      "parent_id": null,
      "is_banned": false,
      "created_at": "2026-05-08T14:00:00Z",
      "User": {
        "ID": 1,
        "Username": "user1"
      },
      "replies": [
        {
          "id": 2,
          "content_id": 1,
          "user_id": 2,
          "text": "回复评论",
          "parent_id": 1,
          "is_banned": false,
          "created_at": "2026-05-08T14:05:00Z",
          "User": {
            "ID": 2,
            "Username": "user2"
          },
          "Parent": {
            "id": 1,
            "user_id": 1,
            "text": "这是一条评论",
            "User": {
              "ID": 1,
              "Username": "user1"
            }
          }
        }
      ]
    }
  ]
}
```

---

### 3. 删除评论

**接口地址**：`DELETE /api/comment/:id`

**描述**：删除指定评论

**认证**：需要登录 + 未被封禁，且只能删除自己的评论（管理员可删除任意评论）

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | uint | 是 | 评论ID（URL路径参数） |

**响应示例**：

```json
{
  "code": 200,
  "message": "删除成功",
  "data": null
}
```

---

### 4. 获取评论数

**接口地址**：`GET /api/comment/count/:content_id`

**描述**：获取指定内容的未封禁评论数量

**认证**：无需登录

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| content_id | uint | 是 | 内容ID（URL路径参数） |

**响应示例**：

```json
{
  "code": 200,
  "message": "获取成功",
  "data": {
    "content_id": 1,
    "count": 10
  }
}
```

---

### 5. 举报评论

**接口地址**：`POST /api/comment/report`

**描述**：举报不当评论

**认证**：需要登录 + 未被封禁

**请求方式**：`application/x-www-form-urlencoded`

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| comment_id | uint | 是 | 评论ID |
| reason | string | 否 | 举报原因（默认"其他"） |

**响应示例**：

```json
{
  "code": 200,
  "message": "举报成功，管理员将尽快处理",
  "data": {
    "id": 1,
    "comment_id": 1,
    "user_id": 2,
    "reason": "广告垃圾",
    "handled": false,
    "created_at": "2026-05-08T15:00:00Z"
  }
}
```

---

### 6. 获取举报列表（管理员）

**接口地址**：`GET /api/admin/comments/reports`

**描述**：获取未处理的评论举报列表

**认证**：需要登录 + 管理员权限

**请求参数**：无

**响应示例**：

```json
{
  "code": 200,
  "message": "获取成功",
  "data": [
    {
      "id": 1,
      "comment_id": 1,
      "reason": "广告垃圾",
      "handled": false,
      "created_at": "2026-05-08T15:00:00Z",
      "Comment": {
        "id": 1,
        "text": "被举报的评论内容"
      },
      "User": {
        "ID": 2,
        "Username": "reporter"
      }
    }
  ]
}
```

---

### 7. 处理举报（管理员）

**接口地址**：`POST /api/admin/comments/reports/:id/handle`

**描述**：标记举报为已处理

**认证**：需要登录 + 管理员权限

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | uint | 是 | 举报ID（URL路径参数） |

**响应示例**：

```json
{
  "code": 200,
  "message": "处理成功",
  "data": {
    "id": 1,
    "handled": true,
    "updated_at": "2026-05-08T16:00:00Z"
  }
}
```

---

## 外置封禁词库

### 文件位置

```
./config/banned_words.json
```

### 文件格式

```json
{
  "words": [
    "封禁词1",
    "封禁词2",
    "敏感词1",
    "敏感词2"
  ]
}
```

### 说明

- 词库文件不存在或解析失败时，关键词过滤功能自动跳过（不影响评论发布）
- 关键词匹配为大小写不敏感

---

## 外置反垃圾API

### 配置位置

```yaml
# config/config.yaml
spam_api:
  url: http://localhost:8081/api/spam/check
```

### 请求格式

```json
POST /api/spam/check
Content-Type: application/json

{
  "text": "评论内容"
}
```

### 响应格式

```json
{
  "code": 966
}
```

### 判断逻辑

| 响应 | 处理 |
|------|------|
| code == 966 | 封禁评论（is_banned = true） |
| code != 966 | 不处理，保持正常 |
| API不可用/超时/解析失败 | 不处理，保持正常 |

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
  "message": "无权删除此评论",
  "data": null
}
```

### 404 评论不存在

```json
{
  "code": 404,
  "message": "评论不存在",
  "data": null
}
```

### 400 请求参数错误

```json
{
  "code": 400,
  "message": "内容包含违规词汇",
  "data": null
}
```

---

## 权限说明

| 操作 | 普通用户 | 管理员 |
|------|----------|--------|
| 添加评论 | ✓（未封禁） | ✓ |
| 删除评论 | ✓（仅自己的） | ✓（任意） |
| 查看评论 | ✓（未封禁） | ✓（全部） |
| 举报评论 | ✓ | ✓ |
| 查看举报 | ✗ | ✓ |
| 处理举报 | ✗ | ✓ |

---

## 评论模型

```json
{
  "id": 1,
  "content_id": 1,
  "user_id": 1,
  "text": "评论内容",
  "parent_id": null,
  "is_banned": false,
  "created_at": "2026-05-08T14:00:00Z",
  "updated_at": "2026-05-08T14:00:00Z",
  "User": {
    "ID": 1,
    "Username": "username"
  },
  "Parent": {
    "id": 1,
    "user_id": 1,
    "text": "父评论内容",
    "User": {
      "ID": 1,
      "Username": "parent_user"
    }
  },
  "replies": []
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 评论ID |
| content_id | uint | 所属内容ID |
| user_id | uint | 评论用户ID |
| text | string | 评论内容 |
| parent_id | uint/null | 父评论ID（回复时使用） |
| is_banned | bool | 是否被封禁 |
| created_at | datetime | 创建时间 |
| updated_at | datetime | 更新时间 |
| User | object | 用户信息 |
| Parent | object | 父评论信息（仅回复评论有，包含父评论的id、user_id、text和User） |
| replies | array | 回复列表（仅一级评论有） |
