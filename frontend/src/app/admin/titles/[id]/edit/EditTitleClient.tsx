'use client'
import { useEffect, useState, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import { usePathname } from 'next/navigation'
import Link from 'next/link'
import NavBar from '@/components/NavBar'
import TagInput from '@/components/TagInput'
import ThumbnailZone from '@/components/ThumbnailZone'

type Title = {
  id: string; title: string; year: number; genre: string[]
  type: 'movie' | 'series'; rating: string; synopsis: string
  director: string; cast: string[]
}

type SaveState = 'idle' | 'saving' | 'done' | 'error'

export default function EditTitleClient({ id: staticId }: { id: string }) {
  // Read real ID from URL (same pattern as WatchClient)
  const pathname = usePathname()
  const parts = pathname.split('/').filter(Boolean)
  const id = parts[parts.indexOf('titles') + 1] ?? staticId

  const router = useRouter()
  const [loading, setLoading] = useState(true)
  const [saveState, setSaveState] = useState<SaveState>('idle')
  const [error, setError] = useState('')

  const [title, setTitle] = useState('')
  const [year, setYear] = useState('')
  const [titleType, setTitleType] = useState<'movie' | 'series'>('movie')
  const [genres, setGenres] = useState<string[]>([])
  const [rating, setRating] = useState('')
  const [synopsis, setSynopsis] = useState('')
  const [director, setDirector] = useState('')

  const [thumbCard, setThumbCard] = useState<{ file: File; preview: string } | null>(null)
  const [thumbPoster, setThumbPoster] = useState<{ file: File; preview: string } | null>(null)
  const [thumbBackdrop, setThumbBackdrop] = useState<{ file: File; preview: string } | null>(null)
  const [existingCard, setExistingCard] = useState<string | null>(null)
  const [existingPoster, setExistingPoster] = useState<string | null>(null)
  const [existingBackdrop, setExistingBackdrop] = useState<string | null>(null)

  useEffect(() => {
    if (!id) return
    fetch(`/api/titles/${id}`)
      .then((r) => r.ok ? r.json() : Promise.reject())
      .then((data: Title) => {
        setTitle(data.title ?? '')
        setYear(String(data.year ?? ''))
        setTitleType(data.type ?? 'movie')
        setGenres(data.genre ?? [])
        setRating(data.rating ?? '')
        setSynopsis(data.synopsis ?? '')
        setDirector(data.director ?? '')
        const probe = (variant: string, setter: (url: string) => void) => {
          fetch(`/api/titles/${id}/thumbnail/${variant}`, { method: 'HEAD' })
            .then((r) => { if (r.ok) setter(`/api/titles/${id}/thumbnail/${variant}`) })
        }
        probe('card', setExistingCard)
        probe('poster', setExistingPoster)
        probe('backdrop', setExistingBackdrop)
      })
      .catch(() => setError('Title not found'))
      .finally(() => setLoading(false))
  }, [id])

  const handleSubmit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault()
    setSaveState('saving')
    setError('')
    const form = new FormData()
    form.append('title', title)
    form.append('year', year)
    form.append('type', titleType)
    form.append('genre', genres.join(','))
    form.append('rating', rating)
    form.append('synopsis', synopsis)
    form.append('director', director)
    if (thumbCard) form.append('card', thumbCard.file)
    if (thumbPoster) form.append('poster', thumbPoster.file)
    if (thumbBackdrop) form.append('backdrop', thumbBackdrop.file)

    try {
      const res = await fetch(`/api/titles/${id}`, { method: 'PUT', body: form })
      if (!res.ok) throw new Error(await res.text())
      setSaveState('done')
      setTimeout(() => router.push(`/title/${id}`), 800)
    } catch (err) {
      setError(String(err))
      setSaveState('error')
    }
  }, [id, title, year, titleType, genres, rating, synopsis, director, thumbCard, thumbPoster, thumbBackdrop, router])

  const busy = saveState === 'saving'
  const inputCls = `w-full bg-[#1c1b1b] border border-[#2a2a2a] rounded-lg px-4 py-2.5 text-[#e5e2e1] text-sm
    focus:outline-none focus:border-[#87a96b]/50 disabled:opacity-50`

  if (loading) return (
    <div className="min-h-screen bg-[#131313]"><NavBar />
      <div className="max-w-2xl mx-auto px-6 pt-28 space-y-4">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="h-11 bg-[#1c1b1b] rounded-lg animate-pulse" />
        ))}
      </div>
    </div>
  )

  if (error && !title) return (
    <div className="min-h-screen bg-[#131313]"><NavBar />
      <div className="max-w-2xl mx-auto px-6 pt-28 text-center text-[#8e9285]">
        <p>{error}</p>
        <Link href="/browse" className="text-[#87a96b] text-sm mt-4 inline-block">← Browse</Link>
      </div>
    </div>
  )

  return (
    <div className="min-h-screen bg-[#131313]">
      <NavBar />
      <main className="max-w-2xl mx-auto px-6 pt-28 pb-16">
        <div className="flex items-center gap-4 mb-8">
          <Link href={`/title/${id}`} className="text-[#8e9285] hover:text-[#e5e2e1] transition-colors text-sm">← Back</Link>
          <h1 className="font-[family-name:var(--font-manrope)] text-3xl font-black text-[#e5e2e1]">Edit title</h1>
        </div>

        {saveState === 'done' && (
          <div className="bg-[#87a96b]/10 border border-[#87a96b]/30 rounded-lg px-4 py-3 text-sm text-[#87a96b] mb-6">
            Saved — redirecting…
          </div>
        )}
        {saveState === 'error' && error && (
          <div className="bg-red-500/10 border border-red-500/30 rounded-lg px-4 py-3 text-sm text-red-400 mb-6">{error}</div>
        )}

        <form onSubmit={handleSubmit} className="space-y-6">
          <div className="grid grid-cols-2 gap-4">
            <div className="col-span-2">
              <label className="block text-xs text-[#8e9285] uppercase tracking-widest mb-1.5">Title</label>
              <input type="text" value={title} onChange={(e) => setTitle(e.target.value)}
                disabled={busy} required className={inputCls} />
            </div>
            <div>
              <label className="block text-xs text-[#8e9285] uppercase tracking-widest mb-1.5">Year</label>
              <input type="number" value={year} onChange={(e) => setYear(e.target.value)}
                disabled={busy} min={1900} max={2099} className={inputCls} />
            </div>
            <div>
              <label className="block text-xs text-[#8e9285] uppercase tracking-widest mb-1.5">Type</label>
              <select value={titleType} onChange={(e) => setTitleType(e.target.value as 'movie' | 'series')}
                disabled={busy} className={inputCls}>
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
              <input type="text" value={rating} onChange={(e) => setRating(e.target.value)}
                disabled={busy} placeholder="TV-MA" className={inputCls} />
            </div>
            <div>
              <label className="block text-xs text-[#8e9285] uppercase tracking-widest mb-1.5">Director</label>
              <input type="text" value={director} onChange={(e) => setDirector(e.target.value)}
                disabled={busy} className={inputCls} />
            </div>
            <div className="col-span-2">
              <label className="block text-xs text-[#8e9285] uppercase tracking-widest mb-1.5">Synopsis</label>
              <textarea value={synopsis} onChange={(e) => setSynopsis(e.target.value)}
                disabled={busy} rows={3} className={`${inputCls} resize-none`} />
            </div>

            <div className="col-span-2">
              <label className="block text-xs text-[#8e9285] uppercase tracking-widest mb-3">
                Visuels <span className="normal-case text-[#454545]">(laisser vide pour conserver l&apos;existant)</span>
              </label>
              <div className="grid grid-cols-3 gap-3">
                <ThumbnailZone label="Card — 16:9" hint="Vignette Browse"
                  aspectClass="aspect-video"
                  preview={thumbCard?.preview ?? existingCard}
                  onChange={(f, p) => setThumbCard({ file: f, preview: p })}
                  onClear={() => { setThumbCard(null); setExistingCard(null) }}
                  disabled={busy} />
                <ThumbnailZone label="Poster — 2:3" hint="Affiche page titre"
                  aspectClass="aspect-[2/3]"
                  preview={thumbPoster?.preview ?? existingPoster}
                  onChange={(f, p) => setThumbPoster({ file: f, preview: p })}
                  onClear={() => { setThumbPoster(null); setExistingPoster(null) }}
                  disabled={busy} />
                <ThumbnailZone label="Backdrop — large" hint="Fond hero"
                  aspectClass="aspect-[21/9]"
                  preview={thumbBackdrop?.preview ?? existingBackdrop}
                  onChange={(f, p) => setThumbBackdrop({ file: f, preview: p })}
                  onClear={() => { setThumbBackdrop(null); setExistingBackdrop(null) }}
                  disabled={busy} />
              </div>
            </div>
          </div>

          <button type="submit" disabled={busy}
            className="w-full bg-[#87a96b] hover:brightness-110 disabled:opacity-40 disabled:cursor-not-allowed text-[#1b3706] font-bold py-3 rounded-full text-sm transition-all active:scale-95">
            {busy ? 'Saving…' : 'Save changes'}
          </button>
        </form>
      </main>
    </div>
  )
}
