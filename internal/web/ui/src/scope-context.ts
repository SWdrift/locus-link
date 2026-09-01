export function useScopeID() {
  const route = useRoute()
  return computed(() => (typeof route.params.scopeId === 'string' ? route.params.scopeId : ''))
}

export function scopePath(scopeID: string, view: 'graph' | 'status' | 'knowledge' | 'inspect') {
  return `/scopes/${encodeURIComponent(scopeID)}/${view}`
}
