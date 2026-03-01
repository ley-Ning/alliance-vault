# Alliance Vault Backend (Go)

这是协作文档工具的 Go 后端 MVP，当前已覆盖文档持久化 + 附件上传链路 + 团队账号管理。

## 技术栈

- Go 1.24
- Gin
- PostgreSQL
- RustFS（S3 兼容对象存储）
- JWT + Refresh Token

## 快速启动

1. 启动依赖服务（PostgreSQL + RustFS）

```bash
docker compose -f docker-compose.dev.yml up -d
```

默认映射端口：
- PostgreSQL：`localhost:55432`
- RustFS API：`localhost:9000`
- RustFS Console：`localhost:9001`

2. 配置环境变量

```bash
cp .env.example .env
```

3. 启动后端

```bash
go mod tidy
go run ./cmd/server
```

默认端口：`http://localhost:8088`

## 账号策略（当前生效）

- 自助注册关闭：`POST /api/v1/auth/register` 固定返回 403
- 默认管理员：`admin / 12345678`（可通过环境变量覆盖）
- 首次登录后必须先改密，改完才可访问文档与管理接口
- 仅管理员可新增团队成员、分配文档权限、禁用/删除账号

## API

- `GET /api/v1/health`：健康检查
- `POST /api/v1/auth/register`：自助注册关闭（返回 403）
- `POST /api/v1/auth/login`：登录并获取会话
- `POST /api/v1/auth/refresh`：刷新令牌
- `GET /api/v1/auth/me`：当前登录用户信息（需鉴权）
- `POST /api/v1/auth/logout`：退出登录（需鉴权）
- `POST /api/v1/auth/change-password`：修改当前账号密码（需鉴权）
- `POST /api/v1/auth/team-members`：管理员新增团队成员（需鉴权，且需先完成改密）
- `GET /api/v1/admin/users`：管理员查看成员列表
- `PATCH /api/v1/admin/users/:id/role`：管理员调整成员角色
- `PATCH /api/v1/admin/users/:id/disabled`：管理员启用/禁用成员
- `DELETE /api/v1/admin/users/:id`：管理员删除成员账号
- `GET /api/v1/admin/documents/:id/permissions`：管理员查看文档专属权限
- `PUT /api/v1/admin/documents/:id/permissions`：管理员分配/更新文档专属权限
- `DELETE /api/v1/admin/documents/:id/permissions/:userId`：管理员移除文档专属权限
- `GET /api/v1/documents`：获取文档列表
- `POST /api/v1/documents`：新建文档
- `GET /api/v1/documents/:id`：获取单篇文档
- `PATCH /api/v1/documents/:id`：更新文档
- `DELETE /api/v1/documents/:id`：删除文档
- `POST /api/v1/uploads/presign`：生成上传签名 URL
- `POST /api/v1/uploads/complete`：上传后确认并写入附件元数据
- `GET /api/v1/attachments/:id`：查询附件元信息
- `GET /api/v1/attachments/:id/download-url`：生成下载签名 URL
- `GET /api/v1/documents/:documentId/attachments`：按文档查询附件列表

## 文档与上传流程

1. 管理员使用默认账号（或已有账号）登录，拿到 `accessToken + refreshToken`
2. 若账号 `mustChangePassword=true`，需先调用 `POST /auth/change-password` 完成改密
3. 文档接口和附件接口统一要求 `Authorization: Bearer <accessToken>`
4. 前端拉取文档列表，编辑后自动调用 `PATCH /documents/:id` 持久化到 PostgreSQL
5. 上传附件时调用 `uploads/presign` 获取 `uploadUrl` 和 `objectKey`
6. 前端使用 `PUT uploadUrl` 上传二进制文件到 RustFS
7. 前端调用 `uploads/complete`，后端校验对象存在后写入 PostgreSQL

删除文档时，服务会同步清理该文档的附件元数据记录。
删除账号时，服务会阻止“删除自己”和“删除最后一个可用管理员”。

## 对象存储验收脚本

```bash
API_BASE_URL=http://localhost:8088 ./scripts/verify-object-storage.sh
```

脚本会自动完成：登录用户 -> 新建文档 -> 申请上传签名 -> 上传 -> 完成回写 -> 申请下载签名 -> 下载比对 -> 清理数据。

> 当前仍使用 `MINIO_*` 变量名，是为了兼容历史配置；实际可直接接 RustFS。
