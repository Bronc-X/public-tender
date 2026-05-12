# 智能招标智能体前端

本目录是智能招标智能体的 React 前端，使用 Vite、TypeScript、Ant Design 和 React Router 构建。

## 启动

```bash
npm install
npm run dev -- --host 127.0.0.1 --port 5173
```

访问：

```text
http://127.0.0.1:5173/
```

后端默认地址为 `http://localhost:8081`，代理配置在 `vite.config.ts`。

## 常用命令

```bash
npm run build   # TypeScript + Vite 生产构建
npm run lint    # ESLint
npm run preview # 预览 dist
```

## 主要页面

- `src/pages/BidProjectWorkbench.tsx`：商务标工作台
- `src/pages/TechBidProjectWorkbench.tsx`：技术标工作台
- `src/pages/FileCenter.tsx`：文件中心
- `src/pages/SystemSettings.tsx`：系统设置
- `src/pages/*Library.tsx`：各类企业资料库
- `src/components/CommerceChapterGenerationPanel.tsx`：商务标章节装配与 Word 导出
- `src/components/OutlineGenerationPanel.tsx`：技术标目录生成
- `src/components/ContentGenerationPanel.tsx`：技术标正文生成

## 交接文档

详见仓库根目录：

- `README.md`
- `docs/开发部署手册.md`
- `docs/用户操作手册.md`
