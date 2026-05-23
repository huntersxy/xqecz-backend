
# 站外资源 API 文档

## 概述

使用 `type=link` 提交B站、抖音、YouTube等视频门户的链接，系统会**自动**检测是否为视频平台链接，并抓取视频标题和封面图，无需手动上传文件。

**支持的平台**：B站(bilibili)、抖音(douyin)、YouTube

---

## 通用说明

### Content 对象字段

| 字段 | 类型 | 说明 |
|------|------|------|
| type | string | 固定为 `link` |
| url | string | 原始视频链接 |
| platform | string | 自动检测的平台：`bilibili` / `douyin` / `youtube`（非视频链接时为空） |
| thumb_path | string | 自动下载的封面图文件名（存储在 uploads/ 目录） |
| image | string | 封面图完整URL（列表/搜索接口返回时自动拼接） |

### 接口通用规则

- 需登录且未被封禁
- 视频链接会被**自动识别**，非阻塞：识别失败不影响内容创建（仅打印日志）
- 自动抓取的标题可被用户提交的 `title` 覆盖
- 封面图下载失败时仅打印警告日志，不影响内容创建
- 提交后 `audit_status` 为 `pending`，需管理员审核

---

## 接口

### 1. 上传站外资源（与上传链接共用接口）

**接口地址**：`POST /api/content/upload`

**请求方式**：`multipart/form-data`

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| title | string | **是** | 标题，留空则自动使用视频标题 |
| type | string | **是** | 固定值 `link` |
| url | string | **是** | 链接，支持普通URL和视频平台URL |
| tags | string[] | 否 | 标签列表 |

**支持的视频URL格式**：

| 平台 | 格式示例 |
|------|---------|
| B站 | `https://www.bilibili.com/video/BV1xx411c7mD` |
| B站(短链) | `https://b23.tv/xxxxx` |
| 抖音 | `https://www.douyin.com/video/7123456789012345678` |
| 抖音(短链) | `https://v.douyin.com/xxxxx` |
| YouTube | `https://www.youtube.com/watch?v=dQw4w9WgXcQ` |
| YouTube(短链) | `https://youtu.be/dQw4w9WgXcQ` |

**请求示例（自动获取标题）**：

```bash
curl -X POST http://localhost:8080/api/content/upload \
  -H "Cookie: session_id=xxx" \
  -F "title=" \
  -F "type=link" \
  -F "url=https://www.bilibili.com/video/BV1GJ411x7h7" \
  -F "tags=动画"
```

**请求示例（自定义标题）**：

```bash
curl -X POST http://localhost:8080/api/content/upload \
  -H "Cookie: session_id=xxx" \
  -F "title=自定义标题" \
  -F "type=link" \
  -F "url=https://www.douyin.com/video/7123456789012345678"
```

**成功响应**：

```json
{
  "code": 200,
  "message": "上传成功",
  "data": {
    "id": 10,
    "title": "【官方MV】周杰伦《晴天》",
    "type": "link",
    "url": "https://www.bilibili.com/video/BV1GJ411x7h7",
    "platform": "bilibili",
    "thumb_path": "1_1715065200000000000_cover.jpg",
    "tags": ["动画"],
    "audit_status": "pending"
  }
}
```

---

### 2. 更新站外资源

**接口地址**：`PUT /api/content/:id`

修改 `url` 时，若新URL为视频链接，系统会重新抓取封面和标题。

```bash
curl -X PUT http://localhost:8080/api/content/10 \
  -H "Cookie: session_id=xxx" \
  -F "url=https://www.bilibili.com/video/BV1es411J7kG" \
  -F "title="
```

---

### 3. 获取站外资源详情

**接口地址**：`GET /api/content/:id`

```json
{
  "code": 200,
  "data": {
    "ID": 10,
    "Title": "【官方MV】周杰伦《晴天》",
    "Type": "link",
    "Url": "https://www.bilibili.com/video/BV1GJ411x7h7",
    "Platform": "bilibili",
    "image": "http://localhost:8080/uploads/1_1715065200000000000_cover.jpg",
    "Tags": ["动画"]
  }
}
```

**字段说明**：

| 字段 | 说明 |
|------|------|
| `Type` | 统一为 `link` |
| `Platform` | 视频平台标识；非视频链接时为空 |
| `image` | 封面图URL，非视频链接时为空 |
| `video` | link类型无本地视频文件，始终为空 |

---

### 4. 列表筛选

`GET /api/content/list?type=link` 可筛选所有链接类型内容（含视频和非视频链接）。

---

## 技术实现说明

### B站(bilibili)

- **API**：`https://api.bilibili.com/x/web-interface/view?bvid={BV号}`
- **提取**：从URL正则匹配 `BV[0-9A-Za-z]{10}`
- **封面**：API返回 `data.pic` 字段，下载到本地

### 抖音(douyin)

- **方式**：请求视频页面HTML，解析 `og:title` / `og:image` meta标签
- **覆盖**：用户提交 `title` 可覆盖自动获取的标题

### YouTube

- **API**：`https://www.youtube.com/oembed?url=...&format=json`
- **封面**：oEmbed 返回 `thumbnail_url`，备用 `img.youtube.com/vi/{id}/maxresdefault.jpg`
