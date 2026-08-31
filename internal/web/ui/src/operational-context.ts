import type { InjectionKey, Reactive } from 'vue'

export interface OperationalContext {
  from: string
  vantage: string
}

const operationalContextKey: InjectionKey<Reactive<OperationalContext>> = Symbol('operational-context')

export function provideOperationalContext(context: Reactive<OperationalContext>) {
  provide(operationalContextKey, context)
}

export function useOperationalContext(): Reactive<OperationalContext> {
  const context = inject(operationalContextKey)
  if (!context) throw new Error('Operational context provider is missing')
  return context
}
