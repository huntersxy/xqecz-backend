# xiaoquan-backend

小泉动漫后端服务，Go + Gin + GORM + MySQL + Redis

## API 文档

- Swagger UI: `http://localhost:8080/swagger/index.html`
- Swagger JSON: `http://localhost:8080/swagger/doc.json`
- 启动服务后，可用 swagger MCP 工具 `fetch_swagger_info` 拉取文档，再用 `list_endpoints` / `execute_api_request` 调测 API

## 常用命令

```bash
go build ./...        # 编译
go vet ./...          # 静态检查
swag init             # 重新生成 Swagger 文档
```

## 项目结构

- `models/` — 数据模型（GORM），Content 含 Platform 字段（link类型视频链接自动检测）
- `handlers/` — 请求处理器（Gin）
- `services/` — 业务服务层，含 external_video.go（视频平台检测/信息抓取/封面下载）
- `utils/` — 工具函数（DB/Redis/安全/错误响应）
- `middleware/` — 中间件（认证/管理员/封禁/错误处理）
- `config/` — 配置加载
- `api文档/` — API 文档
- `docs/` — Swagger 自动生成文档
