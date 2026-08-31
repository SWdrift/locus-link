import { createI18n } from 'vue-i18n'

export type AppLocale = 'zh-CN' | 'en-US'

const messages = {
  'zh-CN': {
    app: { name: 'Locus Link', subtitle: '现场操作上下文', workspace: '工作区', activeScope: '当前 Scope', validated: '已验证', unavailable: '服务不可用', localService: '本机服务' },
    nav: { primary: '主导航', graph: '关系图', status: '状态', knowledge: '知识' },
    settings: { language: '语言', theme: '主题', system: '跟随系统', light: '浅色', dark: '深色' },
    context: { currentEntity: '当前实体', vantage: '观测位置', compact: '运行语境' },
    graph: { eyebrow: '声明视图', title: '操作关系图', entities: '实体', links: 'Link', routes: 'Route', routeList: 'Routes', selectedLink: '已选 Link', probeRoute: 'Probe Route', probeLink: 'Probe Link', resolve: 'Resolve', target: '目标', capability: '能力', resolveRoute: 'Resolve Route', resolveStatus: { resolved: '已解析', unresolved: '未解析', ambiguous: '存在歧义' }, candidates: '{count} 个候选', probeResult: 'Probe {status}', observationsWritten: '写入 {count} 条 Observation', empty: '当前 Scope 中没有可展示的声明图', layout: '正在计算图布局', relayout: '重新布局' },
    common: { loading: '正在加载', noData: '暂无数据', retry: '重试', unknown: '未知', never: '从未', error: '加载失败' },
    status: { eyebrow: 'Observation', title: '状态', description: '显示 {vantage} 下最新的 Link evidence；Route 状态在每次读取时动态派生，不会持久化。', links: 'Link', routes: 'Route', measuredEvidence: '测量 Evidence', derivedStatus: '派生状态', state: '状态', provider: 'Provider', observed: '观测时间', steps: '步骤', success: '成功', failure: '失败', stale: '已过期', unknown: '未知', linkCount: 'Links' },
    knowledge: { eyebrow: '文档', title: '知识', description: '已验证的引用附着于所属声明，并且只从 Scope 的 docs 目录加载。', noDocumentation: '暂无文档', noDocumentationHint: '当前 Registry 没有已验证的文档引用。', references: '{count} 个引用', associations: '{count} 个关联' }
  },
  'en-US': {
    app: { name: 'Locus Link', subtitle: 'Operational context', workspace: 'Workspace', activeScope: 'Active scope', validated: 'Validated', unavailable: 'Unavailable', localService: 'Local service' },
    nav: { primary: 'Primary navigation', graph: 'Graph', status: 'Status', knowledge: 'Knowledge' },
    settings: { language: 'Language', theme: 'Theme', system: 'System', light: 'Light', dark: 'Dark' },
    context: { currentEntity: 'Current entity', vantage: 'Observation vantage', compact: 'Operational context' },
    graph: { eyebrow: 'Declared view', title: 'Operational graph', entities: 'Entities', links: 'Links', routes: 'Routes', routeList: 'Routes', selectedLink: 'Selected link', probeRoute: 'Probe route', probeLink: 'Probe link', resolve: 'Resolve', target: 'Target', capability: 'Capability', resolveRoute: 'Resolve route', resolveStatus: { resolved: 'Resolved', unresolved: 'Unresolved', ambiguous: 'Ambiguous' }, candidates: '{count} candidates', probeResult: 'Probe {status}', observationsWritten: '{count} observations written', empty: 'No declared graph is available in this Scope', layout: 'Calculating graph layout', relayout: 'Relayout' },
    common: { loading: 'Loading', noData: 'No data', retry: 'Retry', unknown: 'Unknown', never: 'Never', error: 'Unable to load' },
    status: { eyebrow: 'Observation', title: 'Status', description: 'Latest Link evidence for {vantage}. Route status is derived on every read and is never persisted.', links: 'Link', routes: 'Route', measuredEvidence: 'Measured evidence', derivedStatus: 'Derived status', state: 'Status', provider: 'Provider', observed: 'Observed', steps: 'Steps', success: 'Success', failure: 'Failure', stale: 'Stale', unknown: 'Unknown', linkCount: 'Links' },
    knowledge: { eyebrow: 'Documentation', title: 'Knowledge', description: 'Validated references remain attached to their declarations and load only from the Scope docs directory.', noDocumentation: 'No documentation', noDocumentationHint: 'This Registry has no validated documentation references.', references: '{count} references', associations: '{count} associations' }
  }
} as const

export const i18n = createI18n({
  legacy: false,
  locale: 'zh-CN',
  fallbackLocale: 'en-US',
  messages,
})
