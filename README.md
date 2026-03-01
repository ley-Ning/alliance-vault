# 项目总览

当前目录已新增一个从 0 到 1 的中文协作文档项目：

- 项目名称：`联盟文舱（Alliance Vault）`
- 项目路径：`/Users/project/notes/alliance-docs`
- 前端技术栈：`React 19 + TypeScript + Vite 7 + Tiptap 3`
- 后端技术栈：`Go 1.24 + Gin + PostgreSQL + MinIO`
- 文档记录：`/Users/project/notes/docs/alliance-docs/从0到1落地记录.md`

## 前端启动

```bash
cd /Users/project/notes/alliance-docs
pnpm install
pnpm dev
```

## 后端启动

```bash
cd /Users/project/notes/alliance-docs/backend
docker compose -f docker-compose.dev.yml up -d
go mod tidy
go run ./cmd/server
```

## 说明

- 前端界面已全部使用中文文案；
- 支持文档新建、删除、搜索、富文本编辑、自动保存、多标签页同步；
- 支持 Markdown 模式与 `.md` 导入导出；
- 已打通文档云端持久化（PostgreSQL），跨浏览器可同步文档内容；
- 后端支持 JWT 登录会话（注册/登录/刷新/退出）；
- 后端支持附件上传签名、上传确认入库、附件列表与下载签名；
- 支持 Electron 桌面端跨平台打包（macOS/Windows/Linux）。
# alliance-vault
