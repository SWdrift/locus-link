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
- 直接集成方：`internal/web/ui/assets.go` 提供可选页面 Handler，`internal/web/server.go`、`internal/web/api.go` 提供 Web API；构建组合以[产物设计](documents/design/产物设计.md)为准。
- 验证边界：前端构建、实际浏览器中的目标流程，以及命中 Web 行为时的工作区本地 E2E CLI 流程。
- 不在本平面内独立改变 Core 领域语义、Registry/Observation 模型、CLI 契约、HTTP 安全边界或 `/api/v0` 版本策略；这些改动必须提升到 `cp-locus-link.md` 协同处理。
- `internal/web/ui/dist/` 是生成产物，不作为手工设计或编辑入口。

## Core Rules

### 设计基线

- 视觉目标按优先级固定为现代、清晰、明确；装饰不与信息竞争，颜色、动效和阴影只表达层级、状态或操作反馈。
- 默认采用紧凑布局：以 `4px` 为基础间距单位，Input、Select、Tag 等常规控件默认使用 Element Plus `small` 密度，正文以 `14px` 为基线；触屏场景通过控件容器和间距保证命中区，不得靠页面各自放大控件。
- 页面使用稳定的标题、工具区、内容区和详情区层级；主要操作保持可见，次要操作就近收纳，危险或有副作用的操作不得只依赖图标表达。
- 颜色、字号、间距、圆角、阴影、层级和状态色必须来自 design token；页面和组件不得继续扩散同义硬编码值。
- 主题提供 `system`、`light`、`dark` 三种模式，默认跟随系统并允许用户显式覆盖；用户选择只保存在浏览器本地，不进入 Core 或 Registry。
- 采用 Element Plus 作为基础组件库，通过静态 theme-chalk CSS、顶层 `ElConfigProvider`、locale 与 CSS variable overrides 统一组件外观和本地化；业务图、operational status 等产品特有表达继续使用窄的自有组件，不套用通用后台模板。
- 应用文案使用 Vue I18n；Element Plus locale 只负责组件内建文案，不替代应用消息目录。
- 组件外观优先使用 Element Plus 的组件、variant、size、state、slot 和 theme token；不得为通用按钮、输入框、选择器、菜单、Card、Alert、Tag、Empty、Skeleton 等重写一套视觉样式。自定义 CSS 只用于应用布局、Graph 等产品特有可视化和组件库无法表达的窄差异。
- 紧凑不是缩小字号或堆叠边框：优先合并重复 Card、复用统一 `PageHeader`/Operational Toolbar、并行展示高频信息，并用 Tabs 等组件收纳互斥详情；页面不得用大块空白代替清晰层级。
- 稳定、跨页面重复的标题、工具栏、状态表达必须组件化；只出现一次且没有独立行为或约束的标记保持就地，避免为“组件化”制造无职责包装层。
- 同一交互层级的 Input、Select 与状态控件必须使用相同 Element Plus size；搜索筛选工具栏只允许 inline/stacked 改变排列，不得建立第二套控件密度。更大尺寸只用于有明确层级差异的主操作，并由公共组件统一承担。
- `PageHeader` 负责页面标题到主体的垂直节奏，页面根布局不得叠加第二份同义 gap。复杂页面只设置一个主工作区 surface；工作区内只能有一个纵向滚动所有者，可展开内容通过增加滚动范围显示，不得压缩相邻 Alert、标题或操作区。
- Tab 切换、筛选、换页和每页数量调整不得改变页面主工作区或同组数据面板的外部高度；数据不足保留稳定空间，数据超出时只在指定内容区内部滚动。
- 搜索与筛选必须放在其直接控制的数据 surface 内；并列但可独立浏览的数据表各自维护筛选与分页状态，不设置脱离数据边界的页面级共享筛选条。
- Border 只用于主 surface 外边界、表格语义网格、分栏边界或必须持续可见的状态分隔；同一 surface 内的标题、筛选与内容优先通过间距、背景层级和排版分组，不连续叠加 divider。
- 基础字号、行高、字重、间距、圆角与稳定布局尺寸集中在 `src/design-tokens.css`，由前端入口 `src/main.ts` 一次引入；reset、主题语义色与应用壳样式留在 `App.vue`，领域样式留在负责该视觉边界的 SFC。不得建立第二套 token、独立页面样式表或无行为的样式包装组件。
- 项目自有 CSS class 使用组件或 feature 前缀的 BEM 风格命名，避免 `panel`、`header`、`field` 等无边界通用名称；只有明确跨组件复用且语义稳定的工具类（当前为 `technical-id`）可以不带组件前缀。Element Plus 与 Vue Flow 的第三方 class 不在此规则内。
- Element Plus 必须通过 `unplugin-vue-components` 与 `unplugin-auto-import` 按需引入；Vue Composition API 同样由 auto-import 提供。只保留 Element Plus dark variables、Vue Flow 基础样式等第三方官方静态 CSS 入口。

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

- Graph 节点布局使用 ELK layered algorithm，并提供显式节点尺寸、方向、分层间距与节点间距；可视 Link 使用 Vue Flow 基于当前 source/target handle 计算的实时正交路径，确保节点拖动时路径和标签持续跟随，优先减少交叉、回折和标签遮挡。
- 布局输入只含稳定声明与必要视觉尺寸；状态、选中态和动画不得改变节点坐标。相同 Graph 投影必须得到确定性位置，刷新 evidence 不触发无意义重排。
- ELK 布局异步执行；实现时优先 Web Worker，并对 Graph 路由和布局引擎延迟加载，避免阻塞初次导航或扩大所有页面的初始成本。
- 断开子图按稳定键分组排列；平行 Link 必须可区分；Route 作为现有 Link overlay 高亮，不创建重复节点或第二套拓扑。
- 首次加载在布局完成后 fit view；后续状态刷新保留用户 viewport。只有拓扑变化或用户显式触发“重新布局”时重算并调整视口。
- Entity 节点允许用户在当前 Graph 画布内拖动以进行临时整理；Vue Flow 在拖动时必须同步重算连接路径与标签位置。拖动只改变当前实例的位置，重新布局、路由重建或刷新后恢复 ELK 确定性布局，不写入 Registry、Observation 或浏览器持久状态。

### 产品与交互

- 前端是 Core 的薄客户端；不得在组件中复制 Resolve、Probe、evidence applicability、Route 状态或声明校验等领域规则。
- Graph、Status、Knowledge 保持同一 active Scope、current Entity 与 vantage 语境；改变语境后，所有相关查询、选择态和反馈必须一致更新。
- Probe 必须由用户显式触发，并清楚展示目标、进行中、成功或失败；页面加载、导航、刷新、Resolve 和读取视图不得隐式 Probe。
- 每项演进先冻结用户任务、入口视图、关键操作、成功反馈、错误态、空态和窄屏行为；不得以无边界视觉重做替代具体目标。
- 桌面侧边栏在 `184px` 展开态与 `56px` 图标态之间切换，折叠入口固定在侧边栏底部，并使用 Element Plus Menu 的公开 collapse API 保持图标居中；折叠状态只属于当前页面实例。移动端保持横向主导航并隐藏无意义的折叠入口。
- 信息优先级服务于 operational context：先呈现对象身份、关系、能力与 evidence，再呈现装饰性信息；Scope 与 Binding 不伪装成 operational graph node。

### 实现边界

- 复用 Vue、Vue Router、TanStack Vue Query 与 Vue Flow；基础组件、静态全局主题和组件 locale 统一使用 Element Plus，应用文案使用 Vue I18n，Graph 自动布局统一使用 ELK。组件样式必须兼容 Web 的 `style-src 'self'` CSP，不引入依赖运行时 style 注入的方案。引入其他依赖前必须证明现有能力无法满足具体需求。
- API 类型与请求集中在 `src/api.ts`；组件不得各自创建第二套 fetch、错误解析或同义 DTO。
- 服务端响应是权威数据；客户端派生值保持可重算，不建立与服务端事实竞争的持久状态。
- Markdown 继续禁用内嵌 HTML并经 DOMPurify 净化；不得通过前端变更扩大外连、脚本、frame、Secret 或 Provider data 暴露面。
- Knowledge 的文本与 Scope 筛选基于已返回的文档索引元数据在客户端完成；Markdown 必须由 `markdown-it` 渲染并继续经 DOMPurify 净化。
- Status 搜索、evidence 状态筛选和分页是完整 Status 投影上的客户端展示派生。`/api/v0/status` 仍返回当前 Registry 的完整 Link/Route 快照供 Graph 和汇总复用；SQLite Observation 存储不等同于可分页事件流，因此不得仅为表格分页拆分该契约。
- Observation 与 Route Evidence 面板使用相同固定高度和固定分页区；表格行高统一，默认每页 10 条。用户切换每页数量时只改变表格内部可见数据和滚动范围，不得改变面板外部高度。
- 破坏 `/api/v0` 字段、语义、副作用或安全边界时，先更新上位设计与契约并迁移所有调用方；不得用前端兼容分支掩盖契约漂移。
- 保持组件职责按用户可见能力划分；出现重复查询、重复状态机或跨视图大段重复标记时，抽取已有稳定概念，不为单次使用创建抽象层。
- 选择或组合 Element Plus 组件时先用公开 props、slots 与 CSS variables；不得依赖 `.el-*` 内部 DOM 结构覆盖样式。确需覆盖时必须说明组件库能力缺口，并将选择器限制在本 feature 根节点。

### 可用性与验证

- 交互元素使用语义化 HTML，支持键盘操作、可见焦点、可读标签和明确的 loading、empty、error、disabled 状态；颜色不得是唯一状态信号。
- 前端代码完成后运行 `pnpm --dir internal/web/ui run build`；命中 Web 可观察行为、API 集成或端到端流程时，还必须运行仓库规定的本地 E2E CLI 流程并保留 `temp/e2e-run/`。
- 视觉或交互变更必须运行实际 Web UI 并用浏览器验证目标流程与目标视口；静态阅读、类型检查或构建成功不能替代视觉验证。
- 修改 Markdown 后运行 `pnpm --dir .tools/markdown run check:links`。
- 可复用的前端决策、验证结论和坑点沉淀到同目录 `cpm-web-ui.md`；一次性进度、临时设计草案和易变代码快照只留在动态任务区。

## Task Board


### 暂缓

- [ ] Home Catalog 与多 Scope 注册、切换和编辑组织：等待 Core、CLI 与 Web 公共契约实现后再接入；当前应用壳只表达服务实际装载的 active Scope。
