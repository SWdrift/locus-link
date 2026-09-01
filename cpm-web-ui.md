# Web UI Memory

本文件保存 `cp-web-ui.md` scope 内已确认、后续前端迭代可复用的决策与理由。当前代码、高优先级规则或控制平面与本文冲突时，以代码、高优先级规则和控制平面为准。

## Design Decisions

- 前端视觉优先级固定为现代、清晰、明确，默认采用紧凑信息密度；紧凑只减少无效留白，不牺牲触屏命中区、可访问性或状态反馈。
- 中文 `zh-CN` 是默认设计与验收语言，英文 `en-US` 同步交付。技术标识保持原值，应用解释与操作文案本地化。
- Element Plus 是基础组件库：静态 theme-chalk CSS 兼容现有 `style-src 'self'` CSP，`ElConfigProvider` 提供 `zh-cn`/`en` 组件 locale，CSS variables 可统一 light/dark 与紧凑 token。
- Vue I18n 管理应用 message catalog；Element Plus locale 只管理组件库内建文案。两者在同一个 locale state 下切换，避免两套语言状态分叉。
- 通用控件视觉由 Element Plus 原生组件与公开 theme token 负责；基础字号、行高、字重、间距、圆角和稳定布局尺寸集中在 `src/design-tokens.css`，其余项目样式留在负责视觉边界的 Vue SFC 中，不建立第二套 token、页面样式表或无行为的样式包装组件。
- 紧凑布局以减少重复容器、重复标题和无效留白为主，不以降低正文可读性实现；跨视图页面标题复用 `PageHeader`，运行语境复用 Operational Toolbar，互斥详情优先由 Element Plus Tabs 等现成组件收纳。
- 主题支持 `system`、`light`、`dark`，默认跟随系统；语言和主题偏好只存浏览器本地，不进入 locus-link Core、Registry 或 Observation。
- Graph 继续由 Vue Flow 渲染，ELK layered algorithm 负责稳定节点布局；Link 由 Vue Flow 基于实时 source/target handle 计算正交路径，因此节点拖动时路径和标签同步移动。Route 和 evidence 仍是现有声明图上的视觉 overlay。
- 项目自有 CSS class 使用组件或 feature 前缀的 BEM 风格；`technical-id` 是当前唯一明确的跨组件工具类例外。Element Plus/Vue Flow 的第三方 class 不纳入项目命名约束。

## Implementation Rationale

- Naive UI 已在实际嵌入式 Web 页面中证明不适用：其运行时 CSS 注入被 `style-src 'self'` CSP 阻止，组件退化为浏览器原生样式。不得放宽 CSP 或加入 `unsafe-inline`；因此改用提供静态 CSS 的 Element Plus。
- ELK layered layout 面向具有固有方向和端口的 node-link graph，支持 Web Worker；相较当前 Dagre 单次同步排布，更适合后续减少交叉、区分平行 Link、处理断开子图并保持 UI 响应。
- ELK 计算可能显著增加 bundle 和 CPU 成本，因此 Graph 路由、布局模块和 worker 应延迟加载；相同拓扑缓存布局，evidence 刷新不得触发重排。
- Element Plus 全局 `small` 尺寸下 Input 为 `32px`、Select 为 `24px`，会造成同一工具栏高度不一致；Operational Toolbar 对 Input、Select、Tag 显式使用 `default`，统一为 `32px`，其余界面继续使用全局紧凑尺寸。
- 响应式验收使用 `360px`、`768px`、`1280px`、`1440px` 四个代表视口，布局在断点之间保持流式；核心 operational context 在任一尺寸均不可被隐藏。
- Graph、Status、Knowledge 路由延迟加载；基础 design token 由 `src/main.ts` 引入 `src/design-tokens.css`，reset、主题语义色与应用壳样式归 `App.vue`，领域样式归负责该视觉边界的 SFC。
- `from + vantage` 由单一 Operational Context provider 管理，Graph 与 Status 复用同一个 Status query key；Knowledge 不再接收无关 context props。
- 实际浏览器已验证 `360px`、`768px`、`1280px`、`1440px` 下三个页面无页面级横向溢出，手机表格只在局部容器滚动；中英文和 system/light/dark 切换会持久化到浏览器本地。
- 前端依赖与命令统一使用 pnpm；Element Plus 组件和 Vue Composition API 分别通过 `unplugin-vue-components`、`unplugin-auto-import` 按需引入，避免手写通用组件 import 与全量 theme-chalk 入口。
- Graph 的 Entity/Link 选择使用判别联合类型，Route overlay 保持独立状态；Inspector 按 Route、Selection、Resolve 三个稳定用户能力拆分，避免把查询、对象详情与操作状态堆进单一大组件。
- Graph 节点拖动只由 Vue Flow 当前实例维护，实时连接路径由 `getSmoothStepPath` 基于当前 handle 坐标计算；刷新、路由重建或显式重新布局后恢复 ELK 位置。该交互不产生持久化或 API。
- Knowledge 搜索 Scope、标题、路径与关联对象元数据；Markdown 由 `markdown-it` 渲染后经 DOMPurify 净化。
- Status 搜索、状态筛选和每表独立分页基于 `/api/v0/status` 完整投影在客户端派生。Observation 与 Route Evidence 面板固定为相同高度，内部表格填充剩余空间，分页 footer 固定高度；SQLite 保存 applicable Observation，不是可分页事件源，因而当前不引入服务端分页。
- 桌面侧边栏使用 `184px`/`56px` 展开与图标态，折叠按钮固定在底部；Element Plus Menu 的 collapse API 负责折叠几何，避免自定义 padding 与组件内部布局竞争。状态不持久化，移动端不显示折叠按钮。
- 查询边界统一复用 `AsyncState`：初次加载使用 Element Plus `ElSkeleton`/`ElSkeletonItem` 按页面结构保留布局，错误使用 `ElResult` 并提供显式重试；后台刷新和业务 mutation 继续保留原内容，只在操作入口展示局部 loading，避免全屏蒙层阻断上下文。

## Case Conventions

- `test/e2e/case/` 的用户可见名称、文档标题和正文以中文展示；canonical ID、Binding role、capability、Provider、路径以及协议字段保持稳定英文标识。
- Web 产物组成以[产物设计](documents/design/产物设计.md)为准，构建与调试入口以 [`scripts/README.md`](scripts/README.md) 为准。
