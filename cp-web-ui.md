# Web UI Control Plane

## Control Plane Role

本控制平面专门管理 `internal/web/ui/` 的前端演进：信息架构、交互、视觉系统、可访问性、响应式行为、客户端状态与 `/api/v0` 的消费方式。它把产品级 locus-link 约束收束为前端可执行规则，但不替代 Web 公共契约、领域设计或代码事实。

本平面从属于 [`cp-locus-link.md`](cp-locus-link.md)。涉及 HTTP 行为、Core 语义、安全边界或 E2E 契约的改动，必须同时受上位控制平面约束；纯前端实现与体验演进由本平面主控。

## Meta Reference

- 入口：非小规模 `internal/web/ui/` 设计、组件、路由、状态管理、数据展示、交互或样式演进。
- 读取顺序：[`cp-meta.md`](cp-meta.md) → [`cpm-locus-link.md`](cpm-locus-link.md) → [`cp-locus-link.md`](cp-locus-link.md) → 本文件 → [`documents/design/contracts/Web契约.md`](documents/design/contracts/Web契约.md) → 相关前端与服务端代码。
- 若存在 `cpm-web-ui.md`，在本文件之前读取；代码和实际运行结果仍是实现现状事实源。
- 创建或扩展 CP 使用 `create-control-plane`；任务或阶段收尾、动态区膨胀时使用 `compact-control-plane`。

## Scope

- 主作用域：`internal/web/ui/src/`、`internal/web/ui/index.html`、Vite 与 TypeScript 配置、前端依赖清单。
- 前端入口：`src/main.ts`、`src/App.vue`、`src/router.ts`；当前视图为 Graph、Status、Knowledge。
- 数据边界：`src/api.ts` 消费本机 `/api/v0`；公共可观察行为以 [`documents/design/contracts/Web契约.md`](documents/design/contracts/Web契约.md) 为准。
- 直接集成方：`internal/web/assets.go` 的构建产物嵌入，以及 `internal/web/server.go`、`internal/web/api.go` 提供的 SPA 与 API。
- 验证边界：前端构建、实际浏览器中的目标流程，以及命中 Web 行为时的工作区本地 E2E CLI 流程。
- 不在本平面内独立改变 Core 领域语义、Registry/Observation 模型、CLI 契约、HTTP 安全边界或 `/api/v0` 版本策略；这些改动必须提升到 `cp-locus-link.md` 协同处理。
- `internal/web/ui/dist/` 是生成产物，不作为手工设计或编辑入口。

## Core Rules

### 设计基线

- 视觉目标按优先级固定为现代、清晰、明确；装饰不与信息竞争，颜色、动效和阴影只表达层级、状态或操作反馈。
- 默认采用紧凑布局：以 `4px` 为基础间距单位，常规控件高度以 `32px` 为基线，正文以 `14px` 为基线；触屏命中区不得因视觉紧凑而小于 `40px`。
- 页面使用稳定的标题、工具区、内容区和详情区层级；主要操作保持可见，次要操作就近收纳，危险或有副作用的操作不得只依赖图标表达。
- 颜色、字号、间距、圆角、阴影、层级和状态色必须来自 design token；页面和组件不得继续扩散同义硬编码值。
- 主题提供 `system`、`light`、`dark` 三种模式，默认跟随系统并允许用户显式覆盖；用户选择只保存在浏览器本地，不进入 Core 或 Registry。
- 采用 Element Plus 作为基础组件库，通过静态 theme-chalk CSS、顶层 `ElConfigProvider`、locale 与 CSS variable overrides 统一组件外观和本地化；业务图、operational status 等产品特有表达继续使用窄的自有组件，不套用通用后台模板。
- 应用文案使用 Vue I18n；Element Plus locale 只负责组件内建文案，不替代应用消息目录。

### 国际化

- 默认 locale 为 `zh-CN`，同时完整提供 `en-US`；语言切换即时生效并持久化到浏览器本地。
- 所有用户可见的应用文案、状态标签、空态、错误提示、操作反馈和可访问名称必须使用稳定 message key，不在模板或业务逻辑中散落中英文字符串。
- canonical ID、Scope ID、Binding role、capability、Provider 名称、路径和原始 evidence 不翻译；其周围的解释、标签与帮助文案翻译。
- 中文是设计和验收基线；英文必须验证文本增长、表格列宽、按钮宽度与窄屏换行，不能只保证 message key 存在。

### 响应式

- 使用内容驱动的流式布局，不以隐藏核心能力作为响应式方案；current Entity、vantage、语言、主题和显式 Probe 在所有受支持尺寸上都必须可访问。
- 至少验证 `360px` 手机、`768px` 平板、`1280px` 桌面和 `1440px` 宽桌面；相邻宽度由弹性 grid、`minmax()`、容器查询或必要的断点连续覆盖。
- 桌面优先紧凑并行工作区；中等尺寸允许 inspector 下移或收起；手机改为单列、抽屉或分步详情。表格和图画布只能在自己的边界内滚动，不得制造页面级横向溢出。
- 信息不得仅因尺寸缩小而消失；确需折叠时必须提供语义明确、支持键盘且保留当前状态的入口。

### Graph 布局

- Graph 自动布局从 Dagre 迁移到 ELK layered algorithm；使用显式节点尺寸、方向、分层间距、节点间距、端口约束和正交 edge routing，优先减少交叉、回折和标签遮挡。
- 布局输入只含稳定声明与必要视觉尺寸；状态、选中态和动画不得改变节点坐标。相同 Graph 投影必须得到确定性位置，刷新 evidence 不触发无意义重排。
- ELK 布局异步执行；实现时优先 Web Worker，并对 Graph 路由和布局引擎延迟加载，避免阻塞初次导航或扩大所有页面的初始成本。
- 断开子图按稳定键分组排列；平行 Link 必须可区分；Route 作为现有 Link overlay 高亮，不创建重复节点或第二套拓扑。
- 首次加载在布局完成后 fit view；后续状态刷新保留用户 viewport。只有拓扑变化或用户显式触发“重新布局”时重算并调整视口。

### 产品与交互

- 前端是 Core 的薄客户端；不得在组件中复制 Resolve、Probe、evidence applicability、Route 状态或声明校验等领域规则。
- Graph、Status、Knowledge 保持同一 active Scope、current Entity 与 vantage 语境；改变语境后，所有相关查询、选择态和反馈必须一致更新。
- Probe 必须由用户显式触发，并清楚展示目标、进行中、成功或失败；页面加载、导航、刷新、Resolve 和读取视图不得隐式 Probe。
- 每项演进先冻结用户任务、入口视图、关键操作、成功反馈、错误态、空态和窄屏行为；不得以无边界视觉重做替代具体目标。
- 信息优先级服务于 operational context：先呈现对象身份、关系、能力与 evidence，再呈现装饰性信息；Scope 与 Binding 不伪装成 operational graph node。

### 实现边界

- 复用 Vue、Vue Router、TanStack Vue Query 与 Vue Flow；基础组件、静态全局主题和组件 locale 统一使用 Element Plus，应用文案使用 Vue I18n，Graph 自动布局统一使用 ELK。组件样式必须兼容 Web 的 `style-src 'self'` CSP，不引入依赖运行时 style 注入的方案。引入其他依赖前必须证明现有能力无法满足具体需求。
- API 类型与请求集中在 `src/api.ts`；组件不得各自创建第二套 fetch、错误解析或同义 DTO。
- 服务端响应是权威数据；客户端派生值保持可重算，不建立与服务端事实竞争的持久状态。
- Markdown 继续禁用内嵌 HTML并经 DOMPurify 净化；不得通过前端变更扩大外连、脚本、frame、Secret 或 Provider data 暴露面。
- 破坏 `/api/v0` 字段、语义、副作用或安全边界时，先更新上位设计与契约并迁移所有调用方；不得用前端兼容分支掩盖契约漂移。
- 保持组件职责按用户可见能力划分；出现重复查询、重复状态机或跨视图大段重复标记时，抽取已有稳定概念，不为单次使用创建抽象层。

### 可用性与验证

- 交互元素使用语义化 HTML，支持键盘操作、可见焦点、可读标签和明确的 loading、empty、error、disabled 状态；颜色不得是唯一状态信号。
- 响应式实现必须遵循本文件的尺寸与能力保留规则；不得产生页面级横向溢出，Graph、表格等需要局部滚动的区域应保持边界明确。
- 视觉或交互变更必须运行实际 Web UI 并用浏览器验证目标流程与目标视口；静态阅读、类型检查或构建成功不能替代视觉验证。
- 前端代码完成后运行 `npm --prefix internal/web/ui run build`；命中 Web 可观察行为、API 集成或端到端流程时，还必须运行仓库规定的本地 E2E CLI 流程并保留 `temp/e2e-run/`。
- 修改 Markdown 后运行 `pnpm --dir .tools/markdown run check:links`。
- 可复用的前端决策、验证结论和坑点沉淀到同目录 `cpm-web-ui.md`；一次性进度、临时设计草案和易变代码快照只留在动态任务区。

## Task Board

当前无已冻结的前端演进任务。后续任务必须从本文件的设计、国际化、响应式和验证规则开始冻结最小边界。
