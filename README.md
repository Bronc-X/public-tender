# 智能招标智能体

面向招投标资料管理、商务标制作、技术标生成和知识库沉淀的一体化工作台。项目采用 Go 后端、React + Vite 前端、SQLite 本地数据库，适合本地私有化演示、二次开发和小团队内部使用。

## 功能概览

- 企业资料库：人员、资质、业绩、荣誉、财报、其他资料的归档与检索。
- 文件中心：上传招标文件、投标资料，执行 OCR/文档解析、审计和归档。
- 商务标制作：招标详情提取、招标规则解析、企业资料适配、资源组合、章节装配确认、Word 导出。
- 技术标制作：项目画像、路线规划、目录生成、目录校验、正文生成、风险复核、定稿导出。
- 系统设置：AI 模型、OCR/文档解析、行业骨架、存储与提示词配置。
- 审计与异常中心：文件审计、问题记录、风险记录和处理状态。

## 技术栈

- 后端：Go + Gin + sqlx + SQLite
- 前端：React 19 + TypeScript + Vite + Ant Design 6
- AI 接入：兼容 DashScope/OpenAI 风格接口，配置保存在本地数据库
- 文档解析：支持本地 OCR 与阿里云文档智能配置

## 快速启动

### 1. 后端

```bash
cd backend_go
go mod download
PORT=8081 APP_DB_PATH=data/app.db go run ./cmd/server
```

后端健康检查：

```bash
curl http://127.0.0.1:8081/health
```

### 2. 前端

```bash
cd frontend
npm install
npm run dev -- --host 127.0.0.1 --port 5173
```

访问：

```text
http://127.0.0.1:5173/
```

Vite 已代理 `/api` 和 `/files` 到 `http://localhost:8081`。

## 常用检查

```bash
# 后端测试
cd backend_go
go test ./...

# 前端类型检查与生产构建
cd frontend
npm run build
```

## 仓库内容说明

本仓库只包含源码和交接文档，不包含运行数据：

- 不包含 `node_modules/`
- 不包含 `dist/`
- 不包含 SQLite 数据库
- 不包含上传文件、导出文件、日志、临时文件和编译产物

首次启动时后端会按 `APP_DB_PATH` 创建本地 SQLite 数据库并执行 schema patch。AI Key、OCR Key、文档解析 Key 请在系统设置页面配置，不要写入代码或提交到仓库。

## 文档入口

- [开发部署手册](docs/开发部署手册.md)
- [用户操作手册](docs/用户操作手册.md)
- [浏览器验证与截图日志](docs/浏览器验证与截图日志_2026-04-28.md)
- [正式跑通阻碍点整改日志](docs/正式跑通阻碍点整改日志_2026-04-27.md)

## 目录结构

```text
.
├── backend_go/          # Go 后端、数据库 schema、服务和测试
├── frontend/            # React + Vite 前端
├── docs/                # 方案、手册、验证记录
├── data/                # 本地运行数据目录，默认不提交
└── README.md
```

## 重要提醒

这是面向招标业务的本地工作台。真实项目资料、招标文件、投标文件、企业证照、数据库和导出成果通常包含敏感信息，应只保存在本地或受控内网环境，不要提交到公开仓库。
