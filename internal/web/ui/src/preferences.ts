import en from 'element-plus/es/locale/lang/en'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import { i18n } from './i18n'
import type { AppLocale } from './i18n'

export type ThemeMode = 'system' | 'light' | 'dark'

const readPreference = <T extends string>(key: string, allowed: readonly T[], fallback: T): T => {
  const value = typeof localStorage === 'undefined' ? null : localStorage.getItem(key)
  return value && allowed.includes(value as T) ? (value as T) : fallback
}

const locale = ref<AppLocale>(readPreference('locus.locale', ['zh-CN', 'en-US'], 'zh-CN'))
const themeMode = ref<ThemeMode>(readPreference('locus.theme', ['system', 'light', 'dark'], 'system'))
const systemDark = ref(typeof matchMedia !== 'undefined' && matchMedia('(prefers-color-scheme: dark)').matches)

if (typeof matchMedia !== 'undefined') {
  matchMedia('(prefers-color-scheme: dark)').addEventListener('change', event => {
    systemDark.value = event.matches
  })
}

watch(
  locale,
  value => {
    i18n.global.locale.value = value
    document.documentElement.lang = value
    localStorage.setItem('locus.locale', value)
  },
  { immediate: true },
)
watch(themeMode, value => localStorage.setItem('locus.theme', value), { immediate: true })

const dark = computed(() => themeMode.value === 'dark' || (themeMode.value === 'system' && systemDark.value))
watch(
  dark,
  value => {
    document.documentElement.classList.toggle('dark', value)
    document.documentElement.style.colorScheme = value ? 'dark' : 'light'
  },
  { immediate: true },
)
const componentLocale = computed(() => (locale.value === 'zh-CN' ? zhCn : en))

export function usePreferences() {
  return { locale, themeMode, dark, componentLocale }
}
