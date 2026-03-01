# 联盟文舱（Alliance Vault）工作区

这是主公当前的项目工作区，核心项目是 `alliance-docs`。

## 目录说明

- 项目源码：`/Users/project/notes/alliance-docs`
- 落地文档：`/Users/project/notes/docs/alliance-docs/从0到1落地记录.md`
- 常用脚本：`/Users/project/notes/scripts`

## 最新能力（已落地）

- 账号策略：关闭自助注册，仅管理员可新增团队成员
- 默认管理员：`admin / 12345678`，首次登录强制修改密码
- 成员管理：管理员可改角色、禁用账号、删除账号（含安全护栏）
- 文档权限：支持按文档分配 `read/edit`，也支持按目录批量分配
- 编辑能力：富文本/Markdown 切换、`.md` 导入导出、中间全屏
- 滚动体验：桌面端编辑器内部滚动；Markdown 双栏滚动联动
- 附件能力：签名上传、列表展示、预览下载
- 桌面端：Electron 跨平台开发与打包（macOS/Windows/Linux）

## 快速启动

### 1) 启动后端依赖

```bash
cd /Users/project/notes/alliance-docs/backend
docker compose -f docker-compose.dev.yml up -d
```

### 2) 启动后端

```bash
cd /Users/project/notes/alliance-docs/backend
cp .env.example .env
go mod tidy
go run ./cmd/server
```

### 3) 启动前端

```bash
cd /Users/project/notes/alliance-docs
cp .env.example .env
pnpm install
pnpm dev --host 127.0.0.1 --port 8080
```

## 说明

- 若本机 `9090` 被占用，可继续使用 `9091` 启动后端联调
- 详细演进记录、实现思路和思维导图都在落地文档里
