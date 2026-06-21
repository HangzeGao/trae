# 前端设计文档 — KVLT Key Vault 管理控制台

> 日期: 2026-06-21  
> 阶段: P0  
> 状态: 已确认

## 1. 概述

为 P0 阶段的 TPM 2.0/vTPM 密钥管理系统实现一个完整的 Web 管理控制台，覆盖密钥管理、节点管理、加密/解密沙盒、DataKey 生成、策略查看、审计日志与健康监控。前端构建产物通过 `go:embed` 嵌入 Go 二进制，单文件部署。

## 2. 技术选型

| 层 | 选型 | 理由 |
|---|---|---|
| 构建 | Vite 5 + React 18 + TypeScript 5 | 快速 HMR，产物可 embed |
| 样式 | Tailwind CSS 4 + 自定义 CSS 变量 | 工具类 + 精确设计控制 |
| 服务端状态 | TanStack Query v5 | 缓存、轮询、乐观更新 |
| 客户端状态 | Zustand | 轻量，管理 auth token |
| 路由 | React Router v6 | SPA 标准方案 |
| 图标 | lucide-react | 轻量、一致 |
| HTTP | 原生 fetch + 拦截器 | 无需 axios |

## 3. 设计语言 — "Cryptographic Instrument"（密码学仪器）

### 3.1 概念

界面应让操作者感觉在使用一台高精度密码学仪器，而非通用 SaaS 后台。灵感来自老式 CRT 磷光显示器、黄铜仪器面板与终端界面，但以现代深色 UI 精致呈现。

### 3.2 色彩系统

```
背景层
  --bg-base:      #0a0a0e   深炭灰（微冷调）
  --bg-surface:   #12121a   凸起表面
  --bg-surface-2: #1a1a26   更高表面
  --bg-inset:     #070709   凹陷区域（代码块/输入框）

边框
  --border:        #252533  常规边框
  --border-bright: #3a3a4d  强调边框

文本
  --text-primary:   #e4e4ec  主文本
  --text-secondary: #9a9aae  次要文本
  --text-tertiary:  #5a5a6e  极弱文本

信号色（仪器指示灯）
  --accent:        #f5a623   琥珀（签名色 / 主操作）
  --accent-bright: #ffb627   亮琥珀（hover/active）
  --success:       #4ade80   磷光绿（ACTIVE/READY）
  --warning:       #fbbf24   黄（DISABLED/DEGRADED/DESTROY_PENDING）
  --danger:        #f87171   红（DESTROYED/REVOKED）
  --info:          #60a5fa   蓝（PRE_ACTIVE/REGISTERED）
```

### 3.3 字体

| 用途 | 字体 | 说明 |
|---|---|---|
| 品牌/页面标题 | Major Mono Display | 全大写等宽，极具辨识度 |
| 技术数据 | JetBrains Mono | hex/base64/nonce/密文展示 |
| UI 正文 | Spline Sans | 现代、清晰、非滥用 |

### 3.4 纹理与氛围

- 全局背景叠加微弱网格图案（`linear-gradient` 网格，opacity 0.03）
- 顶部一条 1px 琥珀色辉光线作为"仪器通电"指示
- 卡片使用极细边框 + 微弱内阴影，营造金属面板质感
- 数据读出区域使用凹陷背景（`--bg-inset`）

### 3.5 状态色映射

| 状态 | 色彩 | 场景 |
|---|---|---|
| ACTIVE / READY | success 绿 | 密钥活跃、节点就绪 |
| DISABLED / DEGRADED / DESTROY_PENDING | warning 黄 | 中间态 |
| DESTROYED / REVOKED | danger 红 | 终态 |
| PRE_ACTIVE / REGISTERED | info 蓝 | 初始态 |

### 3.6 动效

- 页面加载：staggered fade-in（各区块依次出现，80ms 间隔）
- 数据更新：数值变化时短暂高亮
- 状态切换：status pill 脉冲一次
- 按钮悬停：边框增亮 + 微弱琥珀辉光
- 保持克制——这是安全工具，不是营销页

## 4. 页面结构

### 4.1 路由

```
/ui/                  → 登录（Token 输入）
/ui/dashboard         → 总览
/ui/keys              → 密钥列表
/ui/keys/:id          → 密钥详情
/ui/nodes             → 节点列表
/ui/nodes/:id         → 节点详情
/ui/crypto            → 加密/解密沙盒
/ui/data-keys         → DataKey 生成
/ui/policy            → 加密策略查看
/ui/audit             → WAL 审计日志
```

### 4.2 布局

```
┌─────────────────────────────────────────────┐
│ ▔▔▔▔▔ 琥珀辉光线 ▔▔▔▔▔                    │
│ ┌────────┬──────────────────────────────┐  │
│ │        │  面包屑 / 页面标题            │  │
│ │ 侧     │ ───────────────────────────  │  │
│ │ 导     │                              │  │
│ │ 栏     │        主内容区               │  │
│ │        │                              │  │
│ │ 220px  │                              │  │
│ └────────┴──────────────────────────────┘  │
└─────────────────────────────────────────────┘
```

侧边栏固定 220px，含品牌标识 + 导航项 + 底部 token 状态。导航项按平面分组：
- **管理面**：Dashboard、Keys、Nodes
- **数据面**：Crypto、Data Keys
- **参考面**：Policy、Audit

## 5. 页面详细设计

### 5.1 登录页

- 全屏深色背景 + 网格纹理
- 居中卡片，标题 "KEY VAULT" (Major Mono Display)
- 单一输入框：Static Token
- 输入后存入 Zustand + localStorage，fetch 拦截器注入 `Authorization: Bearer`
- 底部小字提示支持的认证方式（Static Token / JWT / HMAC）

### 5.2 Dashboard 总览

四宫格仪表卡 + 系统信息：
- **密钥总数**（含活跃/禁用/销毁待定分布）
- **节点总数**（含就绪/降级/撤销分布）
- **CRK 版本**（当前版本号 + 状态）
- **系统健康**（/healthz 状态 + API 延迟）
- 下方：最近创建的密钥列表（5 条）

### 5.3 密钥列表

表格布局：
- 列：名称、Key ID（等宽截断）、套件、版本、状态（pill）、创建时间
- 顶部：创建密钥按钮（琥珀色主按钮）
- 行点击跳转详情页
- 状态 pill 色彩映射

### 5.4 密钥详情

三区布局：
- **头部**：名称、Key ID、状态 pill、套件徽章、版本号
- **操作区**：按当前状态动态显示按钮
  - ACTIVE → [禁用] [轮转] [计划销毁]
  - DISABLED → [启用]
  - DESTROY_PENDING → 只读提示
  - DESTROYED → 只读提示
- **元数据区**：租户、用途、策略、创建时间、标签
- 危险操作（销毁）需二次确认 modal

### 5.5 节点列表

表格：Node ID、角色、状态 pill、Cluster Epoch、创建时间
- 顶部：注册节点按钮
- 行点击跳转详情

### 5.6 节点详情

- 头部：Node ID、状态 pill、角色
- 基线检查卡片：SELinux、内核版本、虚拟化平台、TPM2-TSS 版本、swtpm 隔离（每项 ✓/✗）
- 操作区：REGISTERED → [标记就绪]；DEGRADED → [撤销]

### 5.7 加密/解密沙盒

双栏布局：
- **左栏 — 加密**
  - 密钥选择器（下拉）
  - 套件显示（只读）
  - 明文输入（textarea，自动 base64 编码）
  - AAD 编辑器（purpose + resource_id 输入）
  - [加密] 按钮 → 输出密文（base64 envelope）
- **右栏 — 解密**
  - 密文输入（textarea，base64）
  - AAD 编辑器
  - [解密] 按钮 → 输出明文（base64 解码 + 原文展示）
- **底部 — Envelope 解析器**
  - 粘贴 base64 envelope → 解析展示 magic/version/suite/key_id/nonce/tag/ciphertext 各字段 hex

### 5.8 DataKey 生成

表单：
- 密钥选择器
- 用途输入
- TTL 滑块（60s — 900s，默认 300s）
- 加密上下文（key-value 动态添加）
- [生成] 按钮 → 结果展示
  - Plaintext DataKey（base64 + hex，带"复制"和"清零"按钮）
  - Wrapped DataKey（base64）
  - 套件、零化截止时间倒计时
  - 加密上下文哈希

### 5.9 策略查看

只读矩阵：
- 默认套件高亮
- 套件列表：Suite ID、算法、密钥位、模式、状态（active/decrypt_only/disabled）
- 状态色彩映射

### 5.10 审计日志

- WAL 条目列表：时间戳、事件 ID、动作（高风险高亮）、目标哈希、操作者哈希、请求 ID
- 高风险动作（CRK 创建/销毁等）用琥珀色左边框标记

## 6. 组件库

| 组件 | 说明 |
|---|---|
| `Layout` | 应用外壳：侧边栏 + 内容区 |
| `Sidebar` | 固定导航，分组 + 激活态 |
| `StatusPill` | 状态徽章，色彩映射 |
| `SuiteBadge` | 套件徽章（AES-256-GCM / SM4-GCM） |
| `MonoReadout` | 等宽数据展示，支持复制、截断展开 |
| `StatCard` | 仪表盘统计卡 |
| `DataTable` | 通用表格（列定义 + 行点击） |
| `Modal` | 二次确认对话框 |
| `Button` | 主（琥珀）/ 次（边框）/ 危险（红） |
| `Input` / `Textarea` | 凹陷背景输入框 |
| `Toast` | 操作反馈通知 |

## 7. API 客户端

```typescript
// fetch 拦截器：自动注入 token + tenant
// 统一错误处理：解析 { error: { code, message, retryable } }
// React Query 封装各端点
```

端点映射（全部相对 `/v1`）：
- `GET /keys` `POST /keys` `GET /keys/:id`
- `POST /keys/:id/enable|disable|rotate|schedule-destroy`
- `GET /nodes/:id` `POST /nodes/register` `POST /nodes/:id/mark-ready|revoke`
- `POST /crypto/encrypt` `POST /crypto/decrypt`
- `POST /data-keys`
- `GET /healthz`

## 8. Go 嵌入方案

```
web/
  frontend/          ← Vite 项目源码
    src/
    package.json
    vite.config.ts   ← base: "/ui/", outDir: "../dist"
  dist/              ← 构建产物（go:embed 目标，gitignore）
internal/web/
  embed.go           ← go:embed dist/*，注册 /ui/* 路由
```

`embed.go`:
- `go:embed` dist 目录
- 注册 `GET /ui/` 与 `GET /ui/*` 路由
- SPA fallback：非文件请求返回 index.html
- 静态资源设置正确 Content-Type + 1 年缓存（hash 文件名）

`server.go` 修改：
- 注入 `WebFS` 到 Deps
- `/ui/*` 路由跳过 auth 中间件

## 9. 构建流程

```bash
# 开发
cd web/frontend && npm install && npm run dev  # Vite dev server :5173, proxy /v1 → :8080

# 生产构建
cd web/frontend && npm run build  # 产物输出到 web/dist/

# Go 构建（embed 前端）
go build -o key-vault ./cmd/key-vault
```

`Makefile` 目标 `make build` 串联前端构建 + Go 构建。

## 10. 验收标准

- [ ] 9 个页面均可访问且功能完整
- [ ] 登录/登出流程正常
- [ ] 密钥 CRUD + 状态机操作可用
- [ ] 加密/解密沙盒端到端可用
- [ ] DataKey 生成 + TTL 倒计时
- [ ] 节点注册/就绪/撤销
- [ ] 策略矩阵只读展示
- [ ] 审计 WAL 回放
- [ ] 前端嵌入 Go 二进制，`/ui/` 可访问
- [ ] 深色主题视觉一致性
- [ ] 响应式（最小 1024px 宽度）
