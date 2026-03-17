import { useEffect, useState } from 'react'

export type Theme = 'dark' | 'light'
const KEY = 'oc-theme'

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(() => {
    return (localStorage.getItem(KEY) as Theme) ?? 'dark'
  })

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
    localStorage.setItem(KEY, theme)
  }, [theme])

  const toggle = () => setTheme((value) => value === 'dark' ? 'light' : 'dark')
  return { theme, toggle }
}
