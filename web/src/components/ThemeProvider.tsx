'use client'
import { useEffect } from 'react'

const STORAGE_KEY = 'spectare-accent'
export const DEFAULT_ACCENT = '#87a96b'

export function getStoredAccent(): string {
  if (typeof window === 'undefined') return DEFAULT_ACCENT
  return localStorage.getItem(STORAGE_KEY) ?? DEFAULT_ACCENT
}

export function applyAccent(color: string) {
  document.documentElement.style.setProperty('--color-accent', color)
  localStorage.setItem(STORAGE_KEY, color)
}

export default function ThemeProvider() {
  useEffect(() => {
    const color = getStoredAccent()
    if (color !== DEFAULT_ACCENT) {
      document.documentElement.style.setProperty('--color-accent', color)
    }
  }, [])
  return null
}
