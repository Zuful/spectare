import { create } from 'zustand'

export interface PlayerTab {
  id: string
  titleId: string
  title: string
  thumbnail: string
  currentTime: number
}

interface PlayerTabsStore {
  tabs: PlayerTab[]
  activeTabId: string | null
  openTab: (tab: Omit<PlayerTab, 'currentTime'>) => void
  closeTab: (id: string) => void
  setActiveTab: (id: string) => void
  updateCurrentTime: (id: string, time: number) => void
}

export const usePlayerTabs = create<PlayerTabsStore>((set, get) => ({
  tabs: [],
  activeTabId: null,

  openTab: (tab) => {
    const existing = get().tabs.find((t) => t.titleId === tab.titleId)
    if (existing) {
      set({ activeTabId: existing.id })
      return
    }
    const newTab: PlayerTab = { ...tab, currentTime: 0 }
    set((s) => ({ tabs: [...s.tabs, newTab], activeTabId: newTab.id }))
  },

  closeTab: (id) => {
    const { tabs, activeTabId } = get()
    const remaining = tabs.filter((t) => t.id !== id)
    const nextActive =
      activeTabId === id ? (remaining[remaining.length - 1]?.id ?? null) : activeTabId
    set({ tabs: remaining, activeTabId: nextActive })
  },

  setActiveTab: (id) => set({ activeTabId: id }),

  updateCurrentTime: (id, time) =>
    set((s) => ({
      tabs: s.tabs.map((t) => (t.id === id ? { ...t, currentTime: time } : t)),
    })),
}))
