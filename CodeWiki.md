# 小泉动漫后端服务 Code Wiki

## 一、项目概述

### 1.1 项目简介

小泉动漫后端服务是一个基于Go语言开发的动漫内容分享平台后端系统，采用Gin作为Web框架，提供用户管理、内容发布、评论互动、审核管理等核心功能。系统支持视频、图片、文字三种类型的内容发布，并配备完善的内容审核机制和垃圾评论过滤功能。

### 1.2 技术栈

本项目采用以下核心技术栈构建：

- **编程语言**：Go 1.25.0
- **Web框架**：Gin v1.12.0
- **ORM框架**：GORM v1.31.1（MySQL驱动 v1.6.0）
- **缓存系统**：Redis v8.11.5
- **配置管理**：YAML v3.0.1
- **密码加密**：bcrypt（golang.org/x/crypto）
- **媒体处理**：FFmpeg（外部依赖）

### 1.3 功能特性

系统具备以下核心功能模块：

- 用户认证与权限管理（注册、登录、登出、会话管理）
- 多类型内容发布（视频、图片、文字，支持文件上传）
- 内容审核工作流（待审核、通过、拒绝三种状态）
- 评论系统（支持回复、举报、管理）
- 标签分类系统
- 内容搜索与推荐
- 视频缩略图自动生成
- 图片压缩处理
- 敏感词过滤
- Redis缓存加速

## 二、项目架构

### 2.1 目录结构

项目采用标准的Go语言分层架构，各目录职责明确：

```
xiaoquan-backend/
├── main.go                    # 应用入口，路由配置
├── go.mod                     # 依赖管理
├── go.sum                     # 依赖锁定
├── config/                    # 配置模块
│   ├── config.go              # 配置结构体与加载逻辑
│   ├── config.yaml            # 配置文件
│   ├── IllegalWords-lexicon.json
│   └── banned_words.json      # 敏感词库
├── models/                    # 数据模型
│   └── models.go              # 数据库模型定义
├── handlers/                   # 业务处理器
│   ├── auth.go                # 认证相关
│   ├── content.go             # 内容管理
│   ├── comment.go             # 评论管理
│   ├── admin.go               # 后台管理
│   └── init_admin.go          # 管理员初始化
├── middleware/                # 中间件
│   ├── auth.go                # 认证中间件
│   └── error.go               # 错误处理中间件
├── utils/                     # 工具函数
│   ├── database.go            # 数据库初始化
│   ├── redis.go               # Redis操作封装
│   ├── security.go            # 安全工具
│   └── video.go               # 视频处理工具
├── uploads/                   # 上传文件存储目录
├── thumbnails/                # 缩略图存储目录
└── scripts/                   # 数据库脚本
```

### 2.2 模块职责划分

系统包含以下五个核心模块，每个模块承担独立的职责：

**config模块**负责应用程序配置管理，包括从YAML文件加载配置、定义配置结构体、构建数据库连接字符串等功能。该模块被其他所有模块依赖，是整个系统的基础。

**models模块**定义了系统所有的数据库模型，包括用户模型、内容模型、评论模型、举报记录模型和审核日志模型。模型层采用GORM框架，提供了数据库表结构与Go结构体之间的映射关系。

**handlers模块**包含所有的HTTP请求处理器，根据业务功能分为认证处理器、内容处理器、评论处理器和管理处理器。每个处理器负责处理特定领域的业务逻辑，包括参数验证、业务处理和响应返回。

**middleware模块**提供了请求处理的中间件，包括身份认证中间件、管理员权限验证中间件、用户封禁检查中间件和全局错误处理中间件。中间件在请求到达处理器之前执行，用于进行权限校验和通用处理。

**utils模块**封装了各种工具函数，包括数据库连接初始化、Redis缓存操作、图片视频处理工具、输入验证工具等。这些工具函数被handlers模块广泛调用，提供可复用的基础能力。

## 三、数据模型详解

### 3.1 模型定义概览

models/models.go文件定义了系统所有的数据模型，采用GORM框架的标签语法进行数据库字段映射。

### 3.2 用户模型（User）

用户模型是系统的基础模型之一，用于存储用户信息。模型定义包含以下字段：ID作为主键自动递增；Username为用户名，创建唯一索引，长度限制50字符且不能为空；Password存储bcrypt加密后的密码，长度255字符；IsAdmin标识用户是否为管理员，默认false；IsBanned标识用户是否被封禁，默认false；CreatedAt和UpdatedAt由GORM自动维护；DeletedAt实现软删除功能。

```go
type User struct {
    ID        uint           `gorm:"primaryKey" json:"id"`
    Username  string         `gorm:"uniqueIndex;size:50;not null" json:"username"`
    Password  string         `gorm:"size:255;not null" json:"-"`
    IsAdmin   bool           `gorm:"default:false" json:"is_admin"`
    IsBanned  bool           `gorm:"default:false" json:"is_banned"`
    CreatedAt time.Time      `json:"created_at"`
    UpdatedAt time.Time      `json:"updated_at"`
    DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
```

### 3.3 内容模型（Content）

内容模型是系统的核心模型，用于存储用户发布的内容。模型支持三种内容类型：视频（video）、图片（image）和文字（text），通过ContentType类型定义枚举常量。

```go
type Content struct {
    ID          uint           `gorm:"primaryKey" json:"id"`
    Title       string         `gorm:"size:200;not null" json:"title"`
    Type        ContentType    `gorm:"size:20;not null;index" json:"type"`
    Content     string         `gorm:"type:text" json:"content,omitempty"`
    FilePath    string         `gorm:"size:500" json:"file_path,omitempty"`
    FileSize    int64          `json:"file_size,omitempty"`
    ThumbPath   string         `gorm:"size:500" json:"thumb_path,omitempty"`
    UserID      uint           `gorm:"not null;index" json:"user_id"`
    User        User           `json:"user,omitempty"`
    BigTagID    *uint          `gorm:"index" json:"big_tag_id,omitempty"`
    SmallTagID  *uint          `gorm:"index" json:"small_tag_id,omitempty"`
    Tags        []string       `gorm:"type:text;serializer:json" json:"tags,omitempty"`
    AuditStatus AuditStatus    `gorm:"size:20;default:pending;index" json:"audit_status"`
    CreatedAt   time.Time      `json:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at"`
    DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}
```

内容模型的Tags字段使用JSON序列化存储，可以保存多个标签。FilePath存储上传文件的路径，ThumbPath存储视频或图片的缩略图路径。BigTagID和SmallTagID用于更细粒度的分类管理（当前代码中未完全实现关联）。AuditStatus字段记录内容的审核状态，包括pending（待审核）、approved（已通过）和rejected（已拒绝）三种状态。

### 3.4 评论模型（Comment）

评论模型支持嵌套回复功能，通过ParentID字段实现评论的层级关系。

```go
type Comment struct {
    ID        uint           `gorm:"primaryKey" json:"id"`
    ContentID uint           `gorm:"not null;index" json:"content_id"`
    Content   Content        `json:"content,omitempty"`
    UserID    uint           `gorm:"not null;index" json:"user_id"`
    User      User           `json:"user,omitempty"`
    Text      string         `gorm:"type:text;not null" json:"text"`
    ParentID  *uint          `gorm:"index" json:"parent_id,omitempty"`
    Parent    *Comment       `gorm:"foreignKey:ParentID;references:ID" json:"parent,omitempty"`
    Replies   []Comment      `gorm:"foreignKey:ParentID;references:ID" json:"replies,omitempty"`
    IsBanned  bool           `gorm:"default:false;index" json:"is_banned"`
    CreatedAt time.Time      `json:"created_at"`
    UpdatedAt time.Time      `json:"updated_at"`
    DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
```

评论支持回复功能，ParentID指向父评论的ID，形成树形结构。IsBanned字段用于标记被系统或管理员封禁的评论，被封禁的评论在前端不显示。

### 3.5 举报记录模型（CommentReport）

举报记录模型用于存储用户对评论的举报信息。

```go
type CommentReport struct {
    ID        uint           `gorm:"primaryKey" json:"id"`
    CommentID uint           `gorm:"not null;index" json:"comment_id"`
    Comment   Comment        `json:"comment,omitempty"`
    UserID    uint           `gorm:"not null;index" json:"user_id"`
    User      User           `json:"user,omitempty"`
    Reason    string         `gorm:"size:255" json:"reason"`
    Handled   bool           `gorm:"default:false" json:"handled"`
    CreatedAt time.Time      `json:"created_at"`
    DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
```

### 3.6 审核日志模型（AuditLog）

审核日志模型记录所有内容审核的操作历史，便于追溯和审计。

```go
type AuditLog struct {
    ID        uint        `gorm:"primaryKey" json:"id"`
    ContentID uint        `gorm:"not null;index" json:"content_id"`
    Content   Content     `json:"content,omitempty"`
    AdminID   uint        `gorm:"not null;index" json:"admin_id"`
    Admin     User        `json:"admin,omitempty"`
    Status    AuditStatus `gorm:"size:20;not null" json:"status"`
    Remark    string      `gorm:"type:text" json:"remark,omitempty"`
    CreatedAt time.Time   `json:"created_at"`
}
```

## 四、核心处理器详解

### 4.1 认证处理器（handlers/auth.go）

认证处理器负责用户的注册、登录、登出和当前用户信息获取功能。

**Register函数**处理用户注册请求。函数首先验证请求参数的完整性，然后检查用户名和密码的格式是否符合要求（用户名3-50字符，密码至少6位）。接下来检查用户名是否已存在，如果存在则返回409冲突错误。使用bcrypt对密码进行加密后创建用户记录。注册成功后返回用户ID。

**Login函数**处理用户登录请求。函数验证用户名和密码的正确性，使用bcrypt的CompareHashAndPassword进行密码比对。登录成功后生成会话ID，将用户ID和会话ID的映射存储到SessionStore中（优先使用Redis，失败则使用内存存储），并设置Cookie返回给客户端。会话ID格式为时间戳加随机字符串，有效期7天。

**Logout函数**处理用户登出请求。从Cookie中获取会话ID，从SessionStore中删除该会话，并将Cookie设置为过期。

**GetMe函数**从请求上下文中获取当前登录用户的信息并返回。中间件已经验证了用户身份并将用户信息设置到上下文中。

**generateSessionID和randomString函数**是认证处理器的辅助函数。generateSessionID生成会话ID，格式为`20060102150405-16位随机字符`。randomString生成指定长度的随机字符串用于会话标识。

### 4.2 内容处理器（handlers/content.go）

内容处理器是系统最复杂的模块，负责内容的发布、查询、修改、删除和审核等功能。

**UploadArticleImage函数**处理文章内图片的上传。用户必须登录才能上传图片。支持的图片格式包括jpg、jpeg、png、gif、webp，单个文件大小限制10MB。图片保存到uploads目录，文件名为`用户ID_时间戳.扩展名`。返回图片的访问URL。

**UploadContent函数**处理内容发布请求。支持三种类型的内容：视频、图片和文字。视频和图片类型需要同时上传文件。标题长度限制1-200字符，内容长度限制10000字符。发布的内容默认处于待审核状态（pending）。对于视频类型，函数会自动调用FFmpeg生成视频封面图。

**GetContentList函数**获取内容列表，支持分页、筛选和排序。函数首先检查Redis缓存，如果缓存命中则直接返回。查询参数包括：audit_status（审核状态）、tag（标签筛选）、type（内容类型）、keyword（关键词搜索）、sort_by（排序字段）、order（排序方向）、page（页码）、page_size（每页数量）。默认只返回已审核通过的内容，按创建时间倒序排列。

**GetMyContentList函数**获取当前用户发布的内容列表，支持按标签和审核状态筛选。

**GetContent函数**获取单个内容的详细信息。函数首先检查Redis缓存，缓存未命中时从数据库查询并更新缓存。缓存有效期12小时。

**UpdateContent函数**修改内容信息。用户只能修改自己发布的内容，管理员可以修改任何内容。修改后内容状态重置为待审核。视频类型支持更换文件，会自动删除旧文件和旧缩略图。

**DeleteContent函数**删除内容及其关联的评论和审核日志。删除前检查权限，用户只能删除自己的内容，管理员可以删除任何内容。删除文件系统的文件和缩略图。

**SearchContent函数**搜索内容，按标题或内容关键词匹配，只返回已审核通过的内容。

**RecommendContent函数**获取随机推荐内容，支持按标签筛选，返回指定数量的内容。

**GetAllTags函数**获取系统中所有已使用的标签列表，用于前端标签选择。

### 4.3 评论处理器（handlers/comment.go）

评论处理器负责评论的添加、查询、删除和举报功能。

**AddComment函数**添加评论。用户必须登录且未被封禁才能评论。评论内容限制5000字符，会检查敏感词过滤。评论关联到指定的内容，且内容必须已审核通过。支持回复功能，通过parent_id指定父评论。添加成功后异步调用垃圾评论检测API。

**checkBannedWords函数**检查文本是否包含敏感词。首先尝试从Redis缓存获取敏感词列表，缓存未命中时从banned_words.json文件加载并缓存24小时。

**asyncCheckSpamAPI函数**异步调用外部垃圾评论检测API。如果API返回code为966，则自动封禁该评论。

**GetComments函数**获取指定内容的评论列表。函数首先检查Redis缓存。返回顶级评论（parent_id为空）及其回复。已封禁的评论不显示。缓存有效期1小时。

**DeleteComment函数**删除评论。用户只能删除自己的评论，管理员可以删除任何评论。删除时同时清理Redis缓存。

**GetCommentCount函数**获取指定内容的评论总数。

**ReportComment函数**举报评论。用户不能举报自己的评论，每个用户对同一评论只能举报一次。举报理由默认为“其他”。

**GetCommentReports函数**获取待处理的举报列表（管理员专用）。

**HandleReport函数**标记举报已处理（管理员专用）。

### 4.4 管理处理器（handlers/admin.go）

管理处理器提供后台管理功能，需要管理员权限。

**AuditContent函数**审核内容。管理员可以批准或拒绝内容。审核状态变更时，如果变为已批准状态，会清理内容列表缓存。同时创建审核日志记录。

**GetPendingContent函数**获取待审核的内容列表。

**GetAllContent函数**获取所有内容列表，支持多条件筛选。

**GetUsers函数**获取用户列表，支持关键词搜索。

**UpdateUserRole函数**修改用户的管理员权限。

**DeleteUser函数**删除用户及其所有发布的内容和关联文件。删除用户时会同时删除该用户的所有内容和评论。

**BanUser函数**封禁或解封用户。管理员不能被封禁。

## 五、中间件详解

### 5.1 认证中间件（middleware/auth.go）

**AuthMiddleware函数**是用户身份认证中间件。函数从Cookie中获取session_id，查询SessionStore获取用户ID。根据用户ID从数据库查询用户信息，设置到请求上下文中。如果会话不存在或用户不存在，返回401未授权错误。

**AdminMiddleware函数**是管理员权限验证中间件。函数首先检查请求上下文中是否存在用户信息，然后检查用户的IsAdmin字段。如果不是管理员，返回403禁止访问错误。

**BannedMiddleware函数**是用户封禁检查中间件。函数检查用户的IsBanned字段。如果用户已被封禁，返回403禁止访问错误。

这三个中间件通常组合使用：`api.Group("/content").Use(middleware.AuthMiddleware(), middleware.BannedMiddleware())`。先验证登录状态，再检查是否被封禁，最后才执行具体的业务处理器。

### 5.2 错误处理中间件（middleware/error.go）

**ErrorHandler函数**是全局错误处理中间件。函数在请求处理完成后检查是否存在错误。如果有错误，返回500内部服务器错误。中间件使用c.Next()允许请求继续传递，在处理完成后统一处理错误。

## 六、工具函数详解

### 6.1 数据库工具（utils/database.go）

**InitDB函数**初始化数据库连接。函数使用配置文件中的MySQL连接信息创建GORM数据库连接。设置连接池参数：最大打开连接数、最大空闲连接数、连接最大生命周期、连接最大空闲时间。然后调用AutoMigrate执行数据库迁移。

**AutoMigrate函数**自动迁移数据库表结构。迁移以下模型：User、Content、Comment、CommentReport、AuditLog。GORM会自动创建不存在的表，更新已存在表的结构。

### 6.2 Redis工具（utils/redis.go）

Redis工具封装了常用的缓存和会话操作。

**InitRedis函数**初始化Redis客户端连接。连接失败时会记录警告但不影响系统运行（系统会降级到内存存储）。

**SetCache和GetCache函数**提供简单的字符串缓存操作。

**SetCacheJSON和GetCacheJSON函数**提供JSON对象的缓存操作，内部自动进行序列化和反序列化。

**DelCache函数**删除指定键的缓存。

**ExistsCache函数**检查指定键是否存在。

**SetSession、GetSession、DeleteSession函数**提供会话管理功能。会话存储在Redis中，有效期24小时。

**ClearUserCache函数**清除指定用户的所有缓存。

**ClearContentCache函数**清除指定内容的所有缓存。

**ClearCommentCache函数**清除指定内容的所有评论相关缓存。

### 6.3 安全工具（utils/security.go）

**SanitizeHTML函数**对HTML特殊字符进行转义，防止XSS攻击。使用html.EscapeString进行转义处理。

**ValidateContentTitle函数**验证内容标题。标题长度必须在1-200字符之间，首尾空格会被去除。

**ValidateTextContent函数**验证文字内容。内容长度不能超过10000字符。

**ValidateUsername函数**验证用户名格式。用户名长度必须在3-50字符之间。

**ValidatePassword函数**验证密码格式。密码长度必须至少6位。

### 6.4 视频处理工具（utils/video.go）

**CheckFFmpeg函数**检查FFmpeg是否可用。FFmpeg是生成视频缩略图的外部依赖。

**GetFFmpegVersion函数**获取FFmpeg版本信息。

**GenerateVideoThumbnail函数**生成视频缩略图。从视频第10帧提取画面，输出为JPEG格式。缩略图文件名格式为`原文件名_thumb.jpg`。

**DeleteVideoThumbnail函数**删除视频缩略图。

**GenerateImageThumbnail函数**生成图片缩略图。将图片缩放至宽度800像素，高度自适应，输出为WebP格式。已存在缩略图时跳过生成。

## 七、配置管理

### 7.1 配置结构（config/config.go）

配置通过YAML文件加载，定义以下配置结构：

**Config结构**是根配置，包含MySQL配置、服务器配置、垃圾评论API配置和Redis配置。

**MySQLConfig结构**定义MySQL连接参数：主机地址、端口、用户名、密码、数据库名、字符集、连接池配置。DSN方法构建MySQL连接字符串。

**ServerConfig结构**定义服务器参数：端口号、上传目录、缩略图目录、最大上传文件大小。

**SpamAPIConfig结构**定义垃圾评论检测API的地址。

**RedisConfig结构**定义Redis连接参数：主机地址、端口、密码、数据库编号、超时时间、键前缀。

### 7.2 配置文件（config/config.yaml）

```yaml
mysql:
  host: 8.134.190.23
  port: 3306
  user: xiaoquan
  password: xiaoquan
  database: xiaoquan
  charset: utf8mb4
  max_open_conns: 30
  max_idle_conns: 10
  conn_max_lifetime: 3600
  conn_max_idle_time: 1800

server:
  port: 8080
  upload_dir: ./uploads
  thumbnail_dir: ./thumbnails
  max_upload_size: 1073741824  # 1GB

spam_api:
  url: http://localhost:8081/api/spam/check

redis:
  host: 8.134.190.23
  port: 6379
  password: "xieying996"
  db: 0
  timeout: 5
  prefix: "xiaoquan:"
```

## 八、API接口设计

### 8.1 认证接口（/api/auth）

| 接口路径 | 方法 | 认证要求 | 功能说明 |
|---------|------|---------|---------|
| /api/auth/register | POST | 无 | 用户注册 |
| /api/auth/login | POST | 无 | 用户登录 |
| /api/auth/logout | POST | 无 | 用户登出 |
| /api/auth/init-admin | POST | 无 | 初始化管理员 |
| /api/auth/me | GET | 必须 | 获取当前用户信息 |

### 8.2 内容接口（/api/content）

| 接口路径 | 方法 | 认证要求 | 功能说明 |
|---------|------|---------|---------|
| /api/content/upload | POST | 必须且未封禁 | 发布内容 |
| /api/content/upload-image | POST | 必须且未封禁 | 上传文章图片 |
| /api/content/list | GET | 无 | 获取内容列表 |
| /api/content/search | GET | 无 | 搜索内容 |
| /api/content/recommend | GET | 无 | 获取推荐内容 |
| /api/content/:id | GET | 无 | 获取内容详情 |
| /api/content/:id | PUT | 必须且未封禁 | 修改内容 |
| /api/content/:id | DELETE | 必须且未封禁 | 删除内容 |
| /api/content/my | GET | 必须 | 获取我的内容列表 |
| /api/content/tags | GET | 无 | 获取所有标签 |

### 8.3 评论接口（/api/comment）

| 接口路径 | 方法 | 认证要求 | 功能说明 |
|---------|------|---------|---------|
| /api/comment/add | POST | 必须且未封禁 | 添加评论 |
| /api/comment/:id | DELETE | 必须且未封禁 | 删除评论 |
| /api/comment/report | POST | 必须且未封禁 | 举报评论 |
| /api/comment/list/:content_id | GET | 无 | 获取评论列表 |
| /api/comment/count/:content_id | GET | 无 | 获取评论数量 |

### 8.4 管理接口（/api/admin）

| 接口路径 | 方法 | 认证要求 | 功能说明 |
|---------|------|---------|---------|
| /api/admin/audit/:id | POST | 必须且是管理员 | 审核内容 |
| /api/admin/pending | GET | 必须且是管理员 | 获取待审核列表 |
| /api/admin/content/all | GET | 必须且是管理员 | 获取所有内容 |
| /api/admin/users | GET | 必须且是管理员 | 获取用户列表 |
| /api/admin/users/:id/role | PUT | 必须且是管理员 | 修改用户角色 |
| /api/admin/users/:id/ban | PUT | 必须且是管理员 | 封禁/解封用户 |
| /api/admin/users/:id | DELETE | 必须且是管理员 | 删除用户 |
| /api/admin/comments/reports | GET | 必须且是管理员 | 获取举报列表 |
| /api/admin/comments/reports/:id/handle | POST | 必须且是管理员 | 处理举报 |
| /api/admin/content/:id/regenerate-thumbnail | POST | 必须且是管理员 | 重新生成缩略图 |

## 九、依赖关系

### 9.1 直接依赖

项目的直接依赖在go.mod中声明：

**github.com/gin-gonic/gin v1.12.0**：轻量级Web框架，提供路由、中间件、参数绑定等功能。

**github.com/go-redis/redis/v8 v8.11.5**：Redis客户端，用于缓存和会话存储。

**gorm.io/driver/mysql v1.6.0**：GORM的MySQL驱动。

**gorm.io/gorm v1.31.1**：Go语言ORM框架，简化数据库操作。

**gopkg.in/yaml.v3 v3.0.1**：YAML配置文件解析库。

**golang.org/x/crypto**：密码加密库，提供bcrypt等加密算法。

### 9.2 间接依赖

Gin框架的间接依赖包括：sonic（JSON序列化）、validator（参数验证）、sse（服务端推送事件）等。

GORM的间接依赖包括：mysql驱动、jinzhu/inflection（复数形式转换）等。

## 十、运行方式

### 10.1 环境要求

运行本项目需要满足以下环境要求：

- Go SDK 1.25.0或更高版本
- MySQL 5.7或更高版本
- Redis 6.0或更高版本
- FFmpeg（用于视频缩略图生成，可选）
- Windows/Linux/macOS操作系统

### 10.2 配置修改

在运行前，需要修改config/config.yaml文件中的配置项：

- MySQL连接信息：host、port、user、password、database
- Redis连接信息：host、port、password
- 服务器端口：port
- 上传目录：根据实际需求配置

### 10.3 编译运行

开发环境直接运行：

```bash
go run main.go
```

Windows环境编译：

```bash
go build -o xiaoquan-backend.exe main.go
./xiaoquan-backend.exe
```

Linux环境编译：

```bash
go build -o xiaoquan-backend main.go
./xiaoquan-backend
```

### 10.4 启动流程

应用启动时执行以下初始化步骤：

1. 加载YAML配置文件
2. 初始化MySQL数据库连接并执行自动迁移
3. 初始化Redis连接（失败时降级到内存存储）
4. 创建uploads和thumbnails目录
5. 检测FFmpeg环境
6. 配置Gin路由和中间件
7. 启动HTTP服务器

### 10.5 目录权限

确保运行程序的用户对以下目录有读写权限：

- ./uploads（上传文件存储）
- ./thumbnails（缩略图存储）
- ./config（配置文件读取）

## 十一、安全机制

### 11.1 密码安全

用户密码使用bcrypt算法加密存储，bcrypt是专为密码哈希设计的算法，具有计算成本高、抗彩虹表攻击等特点。

### 11.2 会话管理

会话ID使用时间戳加随机字符串生成，具有足够的随机性。会话数据优先存储在Redis中，Redis连接失败时降级到内存存储。会话有效期为24小时。

### 11.3 输入验证

系统对所有用户输入进行验证，包括：用户名长度验证、密码长度验证、标题长度验证、内容长度验证、评论长度验证等。

### 11.4 XSS防护

内容标题和正文使用html.EscapeString进行HTML转义，防止跨站脚本攻击。

### 11.5 敏感词过滤

评论内容会检查敏感词列表，敏感词存储在config/banned_words.json文件中，支持从Redis缓存加速。

### 11.6 权限控制

系统实现了三级权限控制：普通用户、封禁用户、管理员。通过AuthMiddleware、BannedMiddleware和AdminMiddleware三个中间件组合实现。

### 11.7 CORS配置

系统配置了CORS中间件，允许跨域请求，支持以下配置：允许来源（支持通配符）、允许凭证、允许的请求头、允许的HTTP方法。
