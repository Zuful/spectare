'use client'
import { useState, useRef, useCallback } from 'react'
import Link from 'next/link'
import NavBar from '@/components/NavBar'
import TagInput from '@/components/TagInput'

type UploadState =
  | { phase: 'idle' }
  | { phase: 'uploading'; uploadPct: number }
  | { phase: 'transcoding'; progress: number }
  | { phase: 'done'; titleId: string }
  | { phase: 'error'; message: string }

export default function UploadPage() {
  const fileRef = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)
  const [dragging, setDragging] = useState(false)
  const [state, setState] = useState<UploadState>({ phase: 'idle' })

  // Metadata fields
  const [title, setTitle] = useState('')
  const [year, setYear] = useState(String(new Date().getFullYear()))
  const [titleType, setTitleType] = useState<'movie' | 'series'>('movie')
  const [genres, setGenres] = useState<string[]>([])
  const [rating, setRating] = useState('')
  const [synopsis, setSynopsis] = useState('')
  const [director, setDirector] = useState('')
  const [doTranscode, setDoTranscode] = useState(false)

  const handleFileDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    setDragging(false)
    const f = e.dataTransfer.files[0]
    if (f) {
      setFile(f)
      if (!title) setTitle(f.name.replace(/\.[^.]+$/, ''))
    }
  }, [title])

  const handleFileInput = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0]
    if (f) {
      setFile(f)
      if (!title) setTitle(f.name.replace(/\.[^.]+$/, ''))
    }
  }, [title])

  const pollStatus = useCallback((id: string) => {
    const interval = setInterval(async () => {
      try {
        const res = await fetch(`/api/titles/${id}/status`)
        const data = await res.json()
        if (data.status === 'ready') {
          clearInterval(interval)
          setState({ phase: 'done', titleId: id })
        } else if (data.status === 'error') {
          clearInterval(interval)
          setState({ phase: 'error', message: data.error || 'Transcoding failed' })
        } else {
          setState({ phase: 'transcoding', progress: Math.round(data.progress ?? 0) })
        }
      } catch {
        clearInterval(interval)
        setState({ phase: 'error', message: 'Lost connection to server' })
      }
    }, 1500)
  }, [])

  const handleSubmit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault()
    if (!file) return

    const form = new FormData()
    form.append('file', file)
    form.append('title', title || file.name)
    form.append('year', year)
    form.append('type', titleType)
    form.append('genre', genres.join(','))
    form.append('rating', rating)
    form.append('synopsis', synopsis)
    form.append('director', director)
    form.append('transcode', doTranscode ? 'true' : 'false')

    setState({ phase: 'uploading', uploadPct: 0 })

    try {
      await new Promise<void>((resolve, reject) => {
        const xhr = new XMLHttpRequest()
        xhr.open('POST', '/api/titles')
        xhr.upload.onprogress = (ev) => {
          if (ev.lengthComputable) {
            setState({ phase: 'uploading', uploadPct: Math.round((ev.loaded / ev.total) * 100) })
          }
        }
        xhr.onload = () => {
          if (xhr.status === 201 || xhr.status === 202) {
            const data = JSON.parse(xhr.responseText)
            if (xhr.status === 202) {
              setState({ phase: 'transcoding', progress: 0 })
              pollStatus(data.id)
            } else {
              setState({ phase: 'done', titleId: data.id })
            }
            resolve()
          } else {
            reject(new Error(xhr.responseText || `HTTP ${xhr.status}`))
          }
        }
        xhr.onerror = () => reject(new Error('Network error'))
        xhr.send(form)
      })
    } catch (err) {
      setState({ phase: 'error', message: String(err) })
    }
  }, [file, title, year, titleType, genres, rating, synopsis, director, pollStatus])

  const busy = state.phase === 'uploading' || state.phase === 'transcoding'

  return (
    <div className="min-h-screen bg-[#131313]">
      <NavBar />
      <main className="max-w-2xl mx-auto px-6 pt-28 pb-16">
        <h1 className="font-[family-name:var(--font-manrope)] text-3xl font-black text-[#e5e2e1] mb-8">
          Add title
        </h1>

        {state.phase === 'done' ? (
          <div className="text-center py-16">
            <div className="text-5xl mb-4">✓</div>
            <p className="text-[#87a96b] text-xl font-semibold mb-2">Ready to watch</p>
            <p className="text-[#8e9285] text-sm mb-8">Your video has been transcoded successfully.</p>
            <div className="flex gap-4 justify-center">
              <Link
                href={`/watch/${state.titleId}`}
                className="bg-[#87a96b] text-[#1b3706] font-bold px-8 py-3 rounded-full text-sm hover:brightness-110 transition-all"
              >
                ▶ Watch now
              </Link>
              <button
                onClick={() => { setFile(null); setTitle(''); setGenres([]); setState({ phase: 'idle' }) }}
                className="border border-[#43483d] text-[#e5e2e1] px-8 py-3 rounded-full text-sm hover:border-[#87a96b]/50 transition-all"
              >
                Add another
              </button>
            </div>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-6">
            {/* Drop zone */}
            <div
              className={`relative border-2 border-dashed rounded-xl p-10 text-center transition-colors ${
                dragging
                  ? 'border-[#87a96b] bg-[#87a96b]/5'
                  : file
                  ? 'border-[#87a96b]/40 bg-[#1c1b1b]'
                  : 'border-[#2a2a2a] hover:border-[#43483d]'
              }`}
              onDragOver={(e) => { e.preventDefault(); setDragging(true) }}
              onDragLeave={() => setDragging(false)}
              onDrop={handleFileDrop}
              onClick={() => !busy && fileRef.current?.click()}
              style={{ cursor: busy ? 'default' : 'pointer' }}
            >
              <input
                ref={fileRef}
                type="file"
                accept="video/*,.mkv,.avi,.mov,.mp4,.m4v,.webm"
                className="hidden"
                onChange={handleFileInput}
                disabled={busy}
              />
              {file ? (
                <div>
                  <p className="text-[#87a96b] font-medium">{file.name}</p>
                  <p className="text-[#8e9285] text-xs mt-1">{(file.size / (1024 * 1024)).toFixed(0)} MB</p>
                  {!busy && <p className="text-[#454545] text-xs mt-2">Click to change</p>}
                </div>
              ) : (
                <div>
                  <div className="text-4xl mb-3 text-[#353534]">⬆</div>
                  <p className="text-[#8e9285] text-sm">Drop a video file here</p>
                  <p className="text-[#454545] text-xs mt-1">MP4, MKV, AVI, MOV, WebM</p>
                </div>
              )}
            </div>

            {/* Progress */}
            {(state.phase === 'uploading' || state.phase === 'transcoding') && (
              <div>
                <div className="flex justify-between text-xs text-[#8e9285] mb-1.5">
                  <span>{state.phase === 'uploading' ? 'Uploading…' : 'Transcoding…'}</span>
                  <span>
                    {state.phase === 'uploading' ? state.uploadPct : state.progress}%
                  </span>
                </div>
                <div className="w-full h-1 bg-[#2a2a2a] rounded-full overflow-hidden">
                  <div
                    className="h-full bg-[#87a96b] transition-all duration-300 rounded-full"
                    style={{ width: `${state.phase === 'uploading' ? state.uploadPct : state.progress}%` }}
                  />
                </div>
                {state.phase === 'transcoding' && state.progress === 0 && (
                  <p className="text-[#454545] text-xs mt-1.5">Starting ffmpeg…</p>
                )}
              </div>
            )}

            {/* Error */}
            {state.phase === 'error' && (
              <div className="bg-red-500/10 border border-red-500/30 rounded-lg px-4 py-3 text-sm text-red-400">
                {state.message}
              </div>
            )}

            {/* Metadata */}
            <div className="grid grid-cols-2 gap-4">
              <div className="col-span-2">
                <label className="block text-xs text-[#8e9285] uppercase tracking-widest mb-1.5">Title</label>
                <input
                  type="text"
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  disabled={busy}
                  className="w-full bg-[#1c1b1b] border border-[#2a2a2a] rounded-lg px-4 py-2.5 text-[#e5e2e1] text-sm focus:outline-none focus:border-[#87a96b]/50 disabled:opacity-50"
                  placeholder="Title name"
                />
              </div>
              <div>
                <label className="block text-xs text-[#8e9285] uppercase tracking-widest mb-1.5">Year</label>
                <input
                  type="number"
                  value={year}
                  onChange={(e) => setYear(e.target.value)}
                  disabled={busy}
                  min={1900}
                  max={2099}
                  className="w-full bg-[#1c1b1b] border border-[#2a2a2a] rounded-lg px-4 py-2.5 text-[#e5e2e1] text-sm focus:outline-none focus:border-[#87a96b]/50 disabled:opacity-50"
                />
              </div>
              <div>
                <label className="block text-xs text-[#8e9285] uppercase tracking-widest mb-1.5">Type</label>
                <select
                  value={titleType}
                  onChange={(e) => setTitleType(e.target.value as 'movie' | 'series')}
                  disabled={busy}
                  className="w-full bg-[#1c1b1b] border border-[#2a2a2a] rounded-lg px-4 py-2.5 text-[#e5e2e1] text-sm focus:outline-none focus:border-[#87a96b]/50 disabled:opacity-50"
                >
                  <option value="movie">Movie</option>
                  <option value="series">Series</option>
                </select>
              </div>
              <div>
                <label className="block text-xs text-[#8e9285] uppercase tracking-widest mb-1.5">Genre</label>
                <TagInput tags={genres} onChange={setGenres} disabled={busy} />
              </div>
              <div>
                <label className="block text-xs text-[#8e9285] uppercase tracking-widest mb-1.5">Rating</label>
                <input
                  type="text"
                  value={rating}
                  onChange={(e) => setRating(e.target.value)}
                  disabled={busy}
                  className="w-full bg-[#1c1b1b] border border-[#2a2a2a] rounded-lg px-4 py-2.5 text-[#e5e2e1] text-sm focus:outline-none focus:border-[#87a96b]/50 disabled:opacity-50"
                  placeholder="TV-MA"
                />
              </div>
              <div>
                <label className="block text-xs text-[#8e9285] uppercase tracking-widest mb-1.5">Director</label>
                <input
                  type="text"
                  value={director}
                  onChange={(e) => setDirector(e.target.value)}
                  disabled={busy}
                  className="w-full bg-[#1c1b1b] border border-[#2a2a2a] rounded-lg px-4 py-2.5 text-[#e5e2e1] text-sm focus:outline-none focus:border-[#87a96b]/50 disabled:opacity-50"
                  placeholder="Christopher Nolan"
                />
              </div>
              <div className="col-span-2">
                <label className="block text-xs text-[#8e9285] uppercase tracking-widest mb-1.5">Synopsis</label>
                <textarea
                  value={synopsis}
                  onChange={(e) => setSynopsis(e.target.value)}
                  disabled={busy}
                  rows={3}
                  className="w-full bg-[#1c1b1b] border border-[#2a2a2a] rounded-lg px-4 py-2.5 text-[#e5e2e1] text-sm focus:outline-none focus:border-[#87a96b]/50 disabled:opacity-50 resize-none"
                  placeholder="A brief description…"
                />
              </div>
            </div>

            {/* Transcode option */}
            <label className={`flex items-start gap-3 p-4 rounded-xl border cursor-pointer transition-colors ${doTranscode ? 'border-[#87a96b]/40 bg-[#87a96b]/5' : 'border-[#2a2a2a] hover:border-[#43483d]'} ${busy ? 'opacity-50 cursor-default' : ''}`}>
              <input
                type="checkbox"
                checked={doTranscode}
                onChange={(e) => !busy && setDoTranscode(e.target.checked)}
                className="mt-0.5 accent-[#87a96b]"
              />
              <div>
                <p className="text-sm font-medium text-[#e5e2e1]">Transcode to HLS <span className="text-[#8e9285] font-normal">(optional)</span></p>
                <p className="text-xs text-[#8e9285] mt-0.5">Generates 360p + 720p adaptive streams via ffmpeg. Slower upload, better playback compatibility across devices. Without this, the file is served directly.</p>
              </div>
            </label>

            <button
              type="submit"
              disabled={!file || busy}
              className="w-full bg-[#87a96b] hover:brightness-110 disabled:opacity-40 disabled:cursor-not-allowed text-[#1b3706] font-bold py-3 rounded-full text-sm transition-all active:scale-95"
            >
              {busy ? 'Processing…' : doTranscode ? 'Upload & transcode' : 'Upload'}
            </button>
          </form>
        )}
      </main>
    </div>
  )
}
