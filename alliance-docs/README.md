# 联盟文舱（Alliance Vault）

联盟文舱是一个面向中文团队的协作文档系统，支持 Web 与桌面端。

## 当前已实现

- 文档工作台：新建、删除、搜索、目录分组、拖拽归档
- 编辑能力：富文本/Markdown 双模式、`.md` 导入导出
- 编辑体验：中间全屏、编辑器内部滚动、Markdown 双栏滚动联动
- 云端持久化：文档存 PostgreSQL，支持离线兜底
- 权限中心：
  - 账号：新增成员、角色切换、启用/禁用、删除账号
  - 文档：按文档分配 `read/edit`，按目录批量分配
- 附件能力：直传 RustFS（S3 兼容）+ 元数据回写 + 预览下载
- 鉴权体系：JWT + Refresh Token + 首次登录强制改密
- 桌面能力：Electron 开发模式与跨平台打包

## 账号策略

- 自助注册关闭：`POST /api/v1/auth/register` 返回 403
- 默认管理员账号：`admin / 12345678`
- 默认管理员首次登录必须修改密码
- 仅管理员可以新增团队成员
- 管理员可以禁用或删除成员账号
- 禁止删除自己，且必须保留至少一个可用管理员

## 技术栈

- 前端：React 19 + TypeScript + Vite 7 + Ant Design + Tiptap 3
- 后端：Go 1.24 + Gin + PostgreSQL + RustFS
- 桌面：Electron + electron-builder

## 本地启动

### 1) 启动后端依赖

```bash
cd /Users/project/notes/alliance-docs/backend
docker compose -f docker-compose.dev.yml up -d
```

### 2) 启动后端服务

```bash
cd /Users/project/notes/alliance-docs/backend
cp .env.example .env
go mod tidy
go run ./cmd/server
```

默认端口：`http://localhost:8088`  
联调常用端口：`http://127.0.0.1:9091`

### 3) 启动前端

```bash
cd /Users/project/notes/alliance-docs
cp .env.example .env
pnpm install
pnpm dev --host 127.0.0.1 --port 8080
```

前端地址：`http://127.0.0.1:8080`

## 桌面端

### 开发模式

```bash
cd /Users/project/notes/alliance-docs
pnpm desktop:dev
```

### 打包

```bash
cd /Users/project/notes/alliance-docs
pnpm desktop:build
```

产物目录：`/Users/project/notes/alliance-docs/release`

## 常用质量命令

```bash
cd /Users/project/notes/alliance-docs
pnpm lint
pnpm build
```

```bash
cd /Users/project/notes/alliance-docs/backend
go test ./...
go build ./...
```

## 文档

- 全量落地记录：`/Users/project/notes/docs/alliance-docs/从0到1落地记录.md`
- 后端接口与策略：`/Users/project/notes/alliance-docs/backend/README.md`
