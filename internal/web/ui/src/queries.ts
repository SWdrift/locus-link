import { useQuery } from '@tanstack/vue-query'
import { getStatus } from './api'
import { useOperationalContext } from './operational-context'
import { useScopeID } from './scope-context'

export const queryKeys = {
  status: (scopeID: string, from: string, vantage: string) => ['status', scopeID, from, vantage] as const,
}

export function useStatusQuery() {
  const context = useOperationalContext()
  const scopeID = useScopeID()
  return useQuery({
    queryKey: computed(() => queryKeys.status(scopeID.value, context.from, context.vantage)),
    queryFn: () => getStatus(scopeID.value, context.from, context.vantage),
    enabled: computed(() => Boolean(scopeID.value && context.vantage)),
  })
}
