# Web UI Memory

本文件保存 `cp-web-ui.md` scope 内已确认、后续前端迭代可复用的决策与理由。当前代码、高优先级规则或控制平面与本文冲突时，以代码、高优先级规则和控制平面为准。

## Design Decisions

- 前端视觉优先级固定为现代、清晰、明确，默认采用紧凑信息密度；紧凑只减少无效留白，不牺牲触屏命中区、可访问性或状态反馈。
- 中文 `zh-CN` 是默认设计与验收语言，英文 `en-US` 同步交付。技术标识保持原值，应用解释与操作文案本地化。
- Element Plus 是基础组件库：静态 theme-chalk CSS 兼容现有 `style-src 'self'` CSP，`ElConfigProvider` 提供 `zh-cn`/`en` 组件 locale，CSS variables 可统一 light/dark 与紧凑 token。
- Vue I18n 管理应用 message catalog；Element Plus locale 只管理组件库内建文案。两者在同一个 locale state 下切换，避免两套语言状态分叉。
- 主题支持 `system`、`light`、`dark`，默认跟随系统；语言和主题偏好只存浏览器本地，不进入 locus-link Core、Registry 或 Observation。
- Graph 继续由 Vue Flow 渲染，但自动布局迁移到 ELK layered algorithm。ELK 只计算稳定位置，Route 和 evidence 仍是现有声明图上的视觉 overlay。

## Implementation Rationale

- Naive UI 已在实际嵌入式 Web 页面中证明不适用：其运行时 CSS 注入被 `style-src 'self'` CSP 阻止，组件退化为浏览器原生样式。不得放宽 CSP 或加入 `unsafe-inline`；因此改用提供静态 CSS 的 Element Plus。
- ELK layered layout 面向具有固有方向和端口的 node-link graph，支持 Web Worker；相较当前 Dagre 单次同步排布，更适合后续减少交叉、区分平行 Link、处理断开子图并保持 UI 响应。
- ELK 计算可能显著增加 bundle 和 CPU 成本，因此 Graph 路由、布局模块和 worker 应延迟加载；相同拓扑缓存布局，evidence 刷新不得触发重排。
- 响应式验收使用 `360px`、`768px`、`1280px`、`1440px` 四个代表视口，布局在断点之间保持流式；核心 operational context 在任一尺寸均不可被隐藏。
- 当前前端已将 Graph、Status、Knowledge 路由延迟加载；页面样式按 feature 拆分，全局样式只保留应用壳、design token、Operational Context 和共享状态。
- `from + vantage` 由单一 Operational Context provider 管理，Graph 与 Status 复用同一个 Status query key；Knowledge 不再接收无关 context props。
- 实际浏览器已验证 `360px`、`768px`、`1280px`、`1440px` 下三个页面无页面级横向溢出，手机表格只在局部容器滚动；中英文和 system/light/dark 切换会持久化到浏览器本地。

## Case Conventions

- `test/e2e/case/` 的用户可见名称、文档标题和正文以中文展示；canonical ID、Binding role、capability、Provider、路径以及协议字段保持稳定英文标识。
- Windows 保留案例 Web UI 的快捷入口是 `scripts/start-test-web.ps1`；默认复用 `temp/e2e-run/`，`-Refresh` 先运行完整 E2E 以重新生成当前源码对应的产物。
