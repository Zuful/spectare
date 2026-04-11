'use client'
import { useEffect, useState, useCallback } from 'react'

type Status = 'idle' | 'remuxing' | 'ready' | 'error'

export default function RemuxButton({ episodeId }: { episodeId: string }) {
  const [status, setStatus] = useState<Status>('idle')
  const [progress, setProgress] = useState(0)
  const [error, setError] = useState('')

  const statusUrl = `/api/episodes/${episodeId}/remux/status`
  const remuxUrl = `/api/episodes/${episodeId}/remux`

  // Check current status on mount
  useEffect(() => {
    fetch(statusUrl)
      .then((r) => r.ok ? r.json() : null)
      .then((data) => {
        if (!data) return
        if (data.status === 'ready') setStatus('ready')
        else if (data.status === 'transcoding') { setStatus('remuxing'); setProgress(data.progress ?? 0) }
      })
      .catch(() => {})
  }, [statusUrl])

  // Poll while remuxing
  useEffect(() => {
    if (status !== 'remuxing') return
    const interval = setInterval(async () => {
      try {
        const res = await fetch(statusUrl)
        const data = await res.json()
        if (data.status === 'ready') { setStatus('ready'); setProgress(100); clearInterval(interval) }
        else if (data.status === 'error') { setStatus('error'); setError(data.error || 'Remux failed'); clearInterval(interval) }
        else if (data.status === 'transcoding') setProgress(Math.round(data.progress ?? 0))
      } catch { clearInterval(interval) }
    }, 1500)
    return () => clearInterval(interval)
  }, [status, statusUrl])

  const handleRemux = useCallback(async () => {
    setStatus('remuxing')
    setProgress(0)
    try {
      await fetch(remuxUrl, { method: 'POST' })
    } catch {
      setStatus('error')
      setError('Failed to start remux')
    }
  }, [remuxUrl])

  if (status === 'ready') {
    return (
      <span className="flex items-center gap-1.5 text-xs text-[var(--color-accent)] font-medium">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"><polyline points="20 6 9 17 4 12"/></svg>
        MP4 ready
      </span>
    )
  }

  if (status === 'remuxing') {
    return (
      <div className="flex items-center gap-3">
        <div className="w-32 h-1 bg-[#2a2a2a] rounded-full overflow-hidden">
          <div className="h-full bg-[var(--color-accent)] transition-all duration-300" style={{ width: `${progress}%` }} />
        </div>
        <span className="text-xs text-[#8e9285] tabular-nums">{progress}%</span>
      </div>
    )
  }

  if (status === 'error') {
    return (
      <div className="flex items-center gap-3">
        <span className="text-xs text-red-400">{error}</span>
        <button onClick={handleRemux} className="text-xs text-[#8e9285] hover:text-[#e5e2e1] underline">Retry</button>
      </div>
    )
  }

  return (
    <button
      onClick={handleRemux}
      className="bg-[var(--color-accent)] hover:brightness-110 text-[#1b3706] font-bold px-4 py-1.5 rounded-full text-xs transition-all active:scale-95 flex items-center gap-1.5"
    >
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
      Convert to MP4
    </button>
  )
}
