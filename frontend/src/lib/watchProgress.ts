const PREFIX_PROGRESS = 'spectare-progress-'
const PREFIX_WATCHED = 'spectare-watched-'

export interface ProgressEntry {
  currentTime: number
  duration: number
  updatedAt: number
}

export function saveProgress(id: string, currentTime: number, duration: number) {
  if (typeof window === 'undefined') return
  if (duration <= 0) return
  const entry: ProgressEntry = { currentTime, duration, updatedAt: Date.now() }
  localStorage.setItem(PREFIX_PROGRESS + id, JSON.stringify(entry))
}

export function getProgress(id: string): ProgressEntry | null {
  if (typeof window === 'undefined') return null
  try {
    const raw = localStorage.getItem(PREFIX_PROGRESS + id)
    return raw ? JSON.parse(raw) : null
  } catch { return null }
}

export function clearProgress(id: string) {
  if (typeof window === 'undefined') return
  localStorage.removeItem(PREFIX_PROGRESS + id)
}

export function markWatched(id: string) {
  if (typeof window === 'undefined') return
  localStorage.setItem(PREFIX_WATCHED + id, '1')
  clearProgress(id)
}

export function isWatched(id: string): boolean {
  if (typeof window === 'undefined') return false
  return localStorage.getItem(PREFIX_WATCHED + id) === '1'
}

export function unmarkWatched(id: string) {
  if (typeof window === 'undefined') return
  localStorage.removeItem(PREFIX_WATCHED + id)
}

const KEY_LAST_PLAYED = 'spectare-last-played'

export function setLastPlayed(id: string) {
  if (typeof window === 'undefined') return
  localStorage.setItem(KEY_LAST_PLAYED, id)
}

export function getLastPlayed(): string | null {
  if (typeof window === 'undefined') return null
  return localStorage.getItem(KEY_LAST_PLAYED)
}

// Returns all IDs with in-progress playback (1%–89% done), newest first
export function getAllInProgress(): Array<{ id: string } & ProgressEntry> {
  if (typeof window === 'undefined') return []
  const results: Array<{ id: string } & ProgressEntry> = []
  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i)
    if (!key?.startsWith(PREFIX_PROGRESS)) continue
    try {
      const entry: ProgressEntry = JSON.parse(localStorage.getItem(key)!)
      const pct = entry.duration > 0 ? entry.currentTime / entry.duration : 0
      if (pct >= 0.01 && pct < 0.9) {
        results.push({ id: key.slice(PREFIX_PROGRESS.length), ...entry })
      }
    } catch {}
  }
  return results.sort((a, b) => b.updatedAt - a.updatedAt)
}
