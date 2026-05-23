# API 响应格式统一指南

## 字段映射

### 内容类型 → 响应字段

| 类型 | 原图/原文件 | 缩略图/封面 | 文本 | 链接 |
|------|-----------|-----------|-----|-----|
| video | `video` → `/uploads/xxx.mp4` | `thumb` → `/thumbnails/xxx.webp` | — | — |
| image | `img` → `/uploads/xxx.jpg` | `thumb` → `/thumbnails/xxx.webp` | — | — |
| text | — | — | `text` → 内容正文 | — |
| link | — | `thumb` → `/thumbnails/xxx.webp`（视频链接时）| — | `url` → 链接地址 |

### 公共字段

| 字段 | 详情接口 | 列表接口 | 说明 |
|------|---------|---------|------|
| `id` | ✓ | ✓ | |
| `title` | ✓ | ✓ | |
| `type` | ✓ | ✓ | video/image/text/link |
| `view_count` | ✓ | ✓ | |
| `tags` | ✓ | ✓ | 永远返回数组（空时 `[]`） |
| `created_at` | ✓ | ✓ | Unix 时间戳（秒） |
| `updated_at` | ✓ | — | |
| `audit_status` | ✓ | — | pending/approved/rejected |
| `user` | ✓ | ✓ | `{id, username, is_admin}` |
| `file_size` | ✓ | — | 仅 video/image |

### 变化总结

| 旧字段 | 新字段 | 说明 |
|-------|-------|------|
| `ID`, `Title`, `Type`... | `id`, `title`, `type`... | PascalCase → snake_case |
| `Content` | `text` | 仅 text 类型 |
| `Url` | `url` | 仅 link 类型 |
| `original` | `img` | 仅 image 类型原图 |
| `image` | `thumb` | 统一缩略图字段（从 `/thumbnails/` 读取） |
| `video` | `video` | 保持不变 |
| `FilePath` | — | 移除，已转为 URL |
| `ThumbPath` | — | 移除，已转为 `image` URL |
| `UserID` | — | 移除，已在 `user.id` 中 |
| `CreatedAt`/`UpdatedAt` | `created_at`/`updated_at` | ISO 字符串 → Unix 时间戳 |
| `User.ID/Username...` | `user.id/username...` | snake_case |
| 空 `content` 返回 `""` | — | 移除，仅在有值时才返回字段 |

---

## 详情接口 vs 列表接口

### 详情接口 (`GET /api/content/:id`)

返回 `buildContentDetail`，包含所有字段。

```json
{
  "id": 1,
  "title": "测试视频",
  "type": "video",
  "video": "http://host/uploads/1_xxx.mp4",
  "thumb": "http://host/thumbnails/1_xxx_thumb.webp",
  "file_size": 102400,
  "view_count": 128,
  "tags": ["动画"],
  "audit_status": "approved",
  "user": { "id": 1, "username": "huntersxy", "is_admin": true },
  "created_at": 1715065200,
  "updated_at": 1715065200
}
```

### 列表接口 (list/search/recommend/my/admin)

返回 `buildContentSummary`，精简字段。

```json
{
  "id": 1,
  "title": "测试视频",
  "type": "video",
  "thumb": "http://host/thumbnails/1_xxx_thumb.webp",
  "view_count": 128,
  "tags": ["动画"],
  "user": { "id": 1, "username": "huntersxy", "is_admin": true },
  "created_at": 1715065200
}
```

---

## 涉及接口

| 接口 | 构建器 | 变更 |
|------|-------|------|
| `GET /api/content/:id` | `buildContentDetail` | 字段名/时间戳/字段精简 |
| `POST /api/content/upload` | `buildContentDetail` | 同上 |
| `PUT /api/content/:id` | `buildContentDetail` | 同上 |
| `GET /api/content/list` | `buildContentSummary` | 同上 + 精简字段 |
| `GET /api/content/search` | `buildContentSummary` | 同上 |
| `GET /api/content/recommend` | `buildContentSummary` | 同上 |
| `GET /api/content/my` | `buildContentSummary` | 同上 |
| `POST /api/admin/audit/:id` | `buildContentDetail` | 同上 |
| `GET /api/admin/pending` | `buildContentSummary` | 同上 |
| `GET /api/admin/content/all` | `buildContentSummary` | 同上 |

---

## 前端适配要点

1. **时间格式**：从 ISO 字符串 `"2026-05-14T15:12:17+08:00"` 改为 Unix 秒 `1715670737`，前端用 `new Date(t * 1000)` 解析
2. **列表不再有 `audit_status`**，需单独获取详情查看审核状态
3. **空数组不再为 null**：`tags` 空时返回 `[]`
4. **字段按类型存在**：`video` 只有 video 类型有，`img` 只有 image 类型有，`text` 只有 text 类型有，`url` 只有 link 类型有
5. **`platform` 字段**：link 类型且为视频链接时返回（bilibili/douyin/youtube），非视频链接时不返回该字段
