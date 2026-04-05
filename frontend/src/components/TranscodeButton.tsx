'use client'
import { useEffect, useState, useCallback } from 'react'

type Status = 'idle' | 'transcoding' | 'ready' | 'error'

export default function TranscodeButton({ titleId }: { titleId: string }) {
  const [status, setStatus] = useState<Status>('idle')
  const [progress, setProgress] = useState(0)
  const [error, setError] = useState('')

  // Poll when transcoding
  useEffect(() => {
    let interval: ReturnType<typeof setInterval> | null = null
    if (status === 'transcoding') {
      interval = setInterval(async () => {
        try {
          const res = await fetch(`/api/titles/${titleId}/status`)
          const data = await res.json()
          if (data.status === 'ready') {
            setStatus('ready')
            setProgress(100)
            clearInterval(interval!)
          } else if (data.status === 'error') {
            setStatus('error')
            setError(data.error || 'Transcoding failed')
            clearInterval(interval!)
          } else if (data.status === 'transcoding') {
            setProgress(Math.round(data.progress ?? 0))
          }
        } catch {
          clearInterval(interval!)
        }
      }, 1500)
    }
    return () => { if (interval) clearInterval(interval) }
  }, [status, titleId])

  // Check current status on mount
  useEffect(() => {
    fetch(`/api/titles/${titleId}/status`)
      .then((r) => r.ok ? r.json() : null)
      .then((data) => {
        if (!data) return
        if (data.status === 'ready') setStatus('ready')
        else if (data.status === 'transcoding') { setStatus('transcoding'); setProgress(data.progress ?? 0) }
      })
      .catch(() => {})
  }, [titleId])

  const handleTranscode = useCallback(async () => {
    setStatus('transcoding')
    setProgress(0)
    try {
      await fetch(`/api/titles/${titleId}/transcode`, { method: 'POST' })
    } catch {
      setStatus('error')
      setError('Failed to start transcoding')
    }
  }, [titleId])

  if (status === 'ready') {
    return (
      <span className="flex items-center gap-1.5 text-xs text-[var(--color-accent)] font-medium">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"><polyline points="20 6 9 17 4 12"/></svg>
        HLS ready
      </span>
    )
  }

  if (status === 'transcoding') {
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
        <button onClick={handleTranscode} className="text-xs text-[#8e9285] hover:text-[#e5e2e1] underline">Retry</button>
      </div>
    )
  }

  return (
    <button
      onClick={handleTranscode}
      className="border border-[#43483d] hover:border-[var(--color-accent)]/50 text-[#8e9285] hover:text-[#e5e2e1] font-medium px-6 py-2.5 rounded-full text-xs transition-all active:scale-95 flex items-center gap-2"
    >
      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polyline points="16 3 21 3 21 8"/><line x1="4" y1="20" x2="21" y2="3"/><polyline points="21 16 21 21 16 21"/><line x1="15" y1="15" x2="21" y2="21"/></svg>
      Transcode to HLS
    </button>
  )
}
