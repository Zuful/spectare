'use client'
import { useEffect, useState, useRef } from 'react'

type Track = { lang: string; label: string; file: string }

const LANG_OPTIONS = [
  { code: 'en', label: 'English' }, { code: 'fr', label: 'Français' },
  { code: 'es', label: 'Español' }, { code: 'de', label: 'Deutsch' },
  { code: 'it', label: 'Italiano' }, { code: 'pt', label: 'Português' },
  { code: 'nl', label: 'Nederlands' }, { code: 'ru', label: 'Русский' },
  { code: 'ja', label: '日本語' }, { code: 'ko', label: '한국어' },
  { code: 'zh', label: '中文' }, { code: 'ar', label: 'العربية' },
]

export default function SubtitleManager({ titleId }: { titleId: string }) {
  const [tracks, setTracks] = useState<Track[]>([])
  const [uploading, setUploading] = useState(false)
  const [lang, setLang] = useState('en')
  const [error, setError] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)

  const load = () => {
    fetch(`/api/titles/${titleId}/subtitles`)
      .then((r) => r.json())
      .then((data) => setTracks(Array.isArray(data) ? data : []))
      .catch(() => {})
  }

  useEffect(() => { load() }, [titleId]) // eslint-disable-line react-hooks/exhaustive-deps

  const handleUpload = async (file: File) => {
    setUploading(true)
    setError('')
    const form = new FormData()
    form.append('file', file)
    form.append('lang', lang)
    try {
      const res = await fetch(`/api/titles/${titleId}/subtitles`, { method: 'POST', body: form })
      if (!res.ok) throw new Error(await res.text())
      load()
    } catch (err) {
      setError(String(err))
    } finally {
      setUploading(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  const handleDelete = async (file: string) => {
    await fetch(`/api/titles/${titleId}/subtitles/${file}`, { method: 'DELETE' })
    load()
  }

  return (
    <div>
      <label className="block text-xs text-[#8e9285] uppercase tracking-widest mb-3">Subtitles</label>

      {/* Existing tracks */}
      {tracks.length > 0 && (
        <div className="space-y-1.5 mb-4">
          {tracks.map((t) => (
            <div key={t.file} className="flex items-center justify-between bg-[#1c1b1b] rounded-lg px-3 py-2">
              <div className="flex items-center gap-2">
                <span className="text-xs font-mono text-[#87a96b] bg-[#87a96b]/10 px-1.5 py-0.5 rounded">{t.lang.toUpperCase()}</span>
                <span className="text-sm text-[#e5e2e1]">{t.label}</span>
                <span className="text-[10px] text-[#454545]">{t.file}</span>
              </div>
              <button
                type="button"
                onClick={() => handleDelete(t.file)}
                className="text-[#454545] hover:text-red-400 transition-colors text-sm px-2"
              >
                ×
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Upload new */}
      <div className="flex gap-2 items-center">
        <select
          value={lang}
          onChange={(e) => setLang(e.target.value)}
          disabled={uploading}
          className="bg-[#1c1b1b] border border-[#2a2a2a] rounded-lg px-3 py-2 text-sm text-[#e5e2e1] focus:outline-none focus:border-[#87a96b]/50 disabled:opacity-50"
        >
          {LANG_OPTIONS.map((l) => (
            <option key={l.code} value={l.code}>{l.label}</option>
          ))}
          <option value="und">Other</option>
        </select>
        <button
          type="button"
          disabled={uploading}
          onClick={() => fileRef.current?.click()}
          className="flex-1 bg-[#1c1b1b] border border-dashed border-[#2a2a2a] hover:border-[#43483d] rounded-lg px-4 py-2 text-sm text-[#8e9285] hover:text-[#e5e2e1] transition-colors disabled:opacity-50 text-left"
        >
          {uploading ? 'Uploading…' : '+ Add .srt or .vtt file'}
        </button>
        <input
          ref={fileRef}
          type="file"
          accept=".srt,.vtt"
          className="hidden"
          onChange={(e) => { const f = e.target.files?.[0]; if (f) handleUpload(f) }}
        />
      </div>

      {error && <p className="text-xs text-red-400 mt-2">{error}</p>}
    </div>
  )
}
