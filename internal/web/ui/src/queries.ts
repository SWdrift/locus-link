import { useQuery } from '@tanstack/vue-query'
import { getStatus } from './api'
import { useOperationalContext } from './operational-context'

export const queryKeys = {
  status: (from: string, vantage: string) => ['status', from, vantage] as const,
}

export function useStatusQuery() {
  const context = useOperationalContext()
  return useQuery({
    queryKey: computed(() => queryKeys.status(context.from, context.vantage)),
    queryFn: () => getStatus(context.from, context.vantage),
    enabled: computed(() => Boolean(context.from && context.vantage)),
  })
}
