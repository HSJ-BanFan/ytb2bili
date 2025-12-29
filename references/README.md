# 参考项目

此目录包含与 ytb2bili 项目相关的外部参考项目和辅助代码。

## 📂 目录结构

```
references/
├── membership/                    # 会员系统前端项目
│   └── ytb2bili-membership-web/   # Next.js 会员管理界面
│       ├── src/
│       ├── public/
│       └── ...
│
├── nextjs-learn-demos/            # Next.js 学习示例
│   ├── app/                       # App Router 示例
│   ├── components/                # 组件示例
│   ├── pages/                     # Pages Router 示例
│   └── ...
│
└── README.md                      # 本文件
```

## 📚 项目说明

### 1. membership/ytb2bili-membership-web

**类型**: 功能模块前端
**技术栈**: Next.js 14 + TypeScript + Tailwind CSS

**功能**:
- 会员等级展示 (免费版/基础版/专业版/企业版)
- 加油包购买
- 配额使用情况
- 支付集成

**关联文档**:
- [会员设计方案](../docs/03-会员系统/01-会员设计方案.md)
- [支付API接口文档](../docs/03-会员系统/06-支付API接口文档.md)

**状态**: ✅ 已集成到主项目 (`web/src/app/(pages)/membership/`)

---

### 2. nextjs-learn-demos/nextjs-learn-demos

**类型**: 学习参考项目
**技术栈**: Next.js 14 (App Router) + TypeScript

**功能**:
- Next.js 14 App Router 示例
- Server Actions 示例
- 客户端/服务端组件示例
- 数据获取模式示例

**用途**:
- 主项目前端开发参考
- Next.js 最佳实践学习
- 组件开发模式参考

**状态**: 📚 学习资源，非生产代码

---

## 🚀 快速开始

### 运行会员系统前端

```bash
cd references/membership/ytb2bili-membership-web
npm install
npm run dev
```

### 运行 Next.js 学习示例

```bash
cd references/nextjs-learn-demos
npm install
npm run dev
```

---

## 📖 使用建议

### 开发参考

1. **学习 Next.js 14 特性**
   - 查看 `nextjs-learn-demos` 的 App Router 示例
   - 参考组件结构和数据获取模式

2. **会员功能开发**
   - 参考 `ytb2bili-membership-web` 的UI设计
   - 了解会员流程和支付集成

3. **代码复用**
   - 从参考项目复制有用的组件
   - 适配到主项目 (`web/` 目录)

---

## 🔗 相关链接

- **主项目前端**: `web/` (Next.js 14)
- **主项目后端**: `internal/` (Go + Gin)
- **文档中心**: `docs/`

---

## ⚠️ 注意事项

1. **不要直接修改参考项目**：这些是独立项目，修改不会影响主项目
2. **主项目已集成**：会员系统功能已集成到 `web/src/app/(pages)/membership/`
3. **版本差异**：参考项目可能与主项目使用不同的依赖版本

---

*最后更新: 2025-12-29*
