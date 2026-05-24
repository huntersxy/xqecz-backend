# 小泉动漫

动漫内容分享社区后端服务，Go + Gin + GORM + MySQL + Redis。

## 快速开始

```bash
# 1. 创建配置文件
cp config/config.example.yaml config/config.yaml
# 编辑 config.yaml，填入 MySQL / Redis 连接信息

# 2. 首次启动创建管理员
# 在 config.yaml 中设置 init_admin: true
go run .

# 3. 查看终端日志获取管理员密码
# 4. 将 init_admin 改回 false，重启

# 5. 访问 Swagger 文档（开发模式）
# http://localhost:8080/swagger/index.html
```

## 配置

所有配置在 `config/config.yaml`，模板见 `config/config.example.yaml`。缺失项首次启动时会自动补齐。

关键开关：

| 配置项 | 说明 |
|--------|------|
| `init_admin` | `true` 时调用 `POST /api/auth/init-admin` 创建管理员（完成后自动关闭） |
| `tinify_enabled` | `true` 启用定时图片压缩（需 `tinify_api_key`） |
| `migrate_thumbnails` | `true` 启动时迁移旧缩略图到 thumbnails/ 目录 |

## 构建

```bash
go build .                        # 开发构建（含 Swagger）
go build -tags noswagger -ldflags="-s -w" -o app .  # 生产构建
```

生产二进制不含 Swagger 依赖，体积 ~28MB。

## 项目结构

```
├── config/          # 配置加载 + 自动补全
├── models/          # GORM 数据模型
├── handlers/        # Gin 路由处理
├── services/        # 业务服务（视频抓取/图片压缩/文件上传）
├── utils/           # 工具（DB/Redis/安全/错误）
├── middleware/       # 中间件（认证/管理员/封禁）
├── scheduler/       # 定时任务（推荐列表/图片压缩）
├── docs/            # Swagger 自动生成
├── api文档/         # API 文档
├── uploads/         # 上传文件目录（运行时）
├── thumbnails/      # 缩略图目录（运行时）
├── images/          # Tinify 压缩图片目录（运行时）
└── config/          # 配置文件
```

## API 文档

- Swagger UI: `http://localhost:8080/swagger/index.html`
- Swagger JSON: `http://localhost:8080/swagger/doc.json`

## 功能

- 内容管理（视频/图片/文字/链接）
- 站外视频聚合（B站/抖音/YouTube 自动抓取封面标题）
- 评论系统（嵌套回复 + 举报审核）
- 投票与内容认领
- 热度推荐引擎
- Tinify 定时图片压缩
- OpenAPI 文档 + MCP 集成

## 许可

MIT
