'use client'
import { useEffect, useState, useRef } from 'react'
import { usePathname } from 'next/navigation'
import Link from 'next/link'
import NavBar from '@/components/NavBar'
import TranscodeButton from '@/components/TranscodeButton'

type TitleData = {
  id: string; title: string; year: number; genre: string[]
  type: string; rating: string; synopsis: string; director: string
  streamReady: boolean; directPath?: string
}

type Episode = {
  id: string
  seriesId: string
  season: number
  number: number
  title: string
  synopsis: string
  directPath?: string
  streamReady: boolean
  transcodeStatus: 'pending' | 'transcoding' | 'ready' | 'error'
  createdAt: string
}

function EpisodeItem({ ep }: { ep: Episode }) {
  const label = `S${String(ep.season).padStart(2, '0')}E${String(ep.number).padStart(2, '0')}`
  return (
    <div className="flex gap-4 p-3 rounded-lg bg-[#1a1a1a] hover:bg-[#1e1e1e] border border-[#2a2a2a] hover:border-[#3a3a3a] transition-all">
      {/* Thumbnail */}
      <div className="flex-shrink-0 w-32 aspect-video bg-[#0e0e0e] rounded overflow-hidden">
        <img
          src={`/api/episodes/${ep.id}/thumbnail`}
          alt=""
          className="w-full h-full object-cover"
          onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
        />
      </div>
      {/* Info */}
      <div className="flex-1 min-w-0 py-0.5">
        <div className="flex items-start justify-between gap-3 mb-1">
          <div className="min-w-0">
            <span className="text-xs font-mono text-[var(--color-accent)] mr-2">{label}</span>
            <span className="text-sm font-semibold text-[#e5e2e1]">{ep.title}</span>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            {!ep.streamReady && (
              <TranscodeButton titleId={ep.seriesId} episodeId={ep.id} />
            )}
            <Link
              href={`/watch/episode/${ep.id}`}
              className="bg-[var(--color-accent)] hover:brightness-110 text-[#1b3706] font-bold px-4 py-1.5 rounded-full text-xs transition-all active:scale-95 flex items-center gap-1"
            >
              ▶ Play
            </Link>
          </div>
        </div>
        {ep.synopsis && (
          <p className="text-xs text-[#8e9285] leading-relaxed line-clamp-2">{ep.synopsis}</p>
        )}
      </div>
    </div>
  )
}

function AddEpisodeForm({ id, onAdded }: { id: string; onAdded: () => void }) {
  const [open, setOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const formRef = useRef<HTMLFormElement>(null)

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setError('')
    setSubmitting(true)
    const form = e.currentTarget
    const fd = new FormData(form)
    try {
      const res = await fetch(`/api/titles/${id}/episodes`, { method: 'POST', body: fd })
      if (!res.ok) {
        const txt = await res.text().catch(() => 'Unknown error')
        throw new Error(txt)
      }
      form.reset()
      setOpen(false)
      onAdded()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add episode')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="mt-4">
      <button
        onClick={() => setOpen((v) => !v)}
        className="border border-[#43483d] hover:border-[var(--color-accent)]/50 text-[#8e9285] hover:text-[#e5e2e1] font-medium px-6 py-2.5 rounded-full text-sm transition-all active:scale-95 flex items-center gap-2"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
        Add Episode
      </button>

      {open && (
        <form ref={formRef} onSubmit={handleSubmit} className="mt-4 p-5 bg-[#1a1a1a] border border-[#2a2a2a] rounded-xl flex flex-col gap-4 max-w-lg">
          <h3 className="font-[family-name:var(--font-manrope)] text-base font-bold text-[#e5e2e1]">New Episode</h3>

          <div className="grid grid-cols-2 gap-3">
            <label className="flex flex-col gap-1">
              <span className="text-xs text-[#8e9285] uppercase tracking-widest">Season</span>
              <input
                name="season"
                type="number"
                min="1"
                required
                defaultValue="1"
                className="bg-[#0e0e0e] border border-[#2a2a2a] focus:border-[var(--color-accent)]/50 rounded-lg px-3 py-2 text-sm text-[#e5e2e1] outline-none transition-colors"
              />
            </label>
            <label className="flex flex-col gap-1">
              <span className="text-xs text-[#8e9285] uppercase tracking-widest">Episode</span>
              <input
                name="number"
                type="number"
                min="1"
                required
                defaultValue="1"
                className="bg-[#0e0e0e] border border-[#2a2a2a] focus:border-[var(--color-accent)]/50 rounded-lg px-3 py-2 text-sm text-[#e5e2e1] outline-none transition-colors"
              />
            </label>
          </div>

          <label className="flex flex-col gap-1">
            <span className="text-xs text-[#8e9285] uppercase tracking-widest">Title</span>
            <input
              name="title"
              type="text"
              required
              placeholder="Episode title"
              className="bg-[#0e0e0e] border border-[#2a2a2a] focus:border-[var(--color-accent)]/50 rounded-lg px-3 py-2 text-sm text-[#e5e2e1] outline-none transition-colors placeholder:text-[#454545]"
            />
          </label>

          <label className="flex flex-col gap-1">
            <span className="text-xs text-[#8e9285] uppercase tracking-widest">Synopsis <span className="normal-case text-[#454545]">(optional)</span></span>
            <textarea
              name="synopsis"
              rows={3}
              placeholder="Brief description..."
              className="bg-[#0e0e0e] border border-[#2a2a2a] focus:border-[var(--color-accent)]/50 rounded-lg px-3 py-2 text-sm text-[#e5e2e1] outline-none transition-colors placeholder:text-[#454545] resize-none"
            />
          </label>

          <label className="flex flex-col gap-1">
            <span className="text-xs text-[#8e9285] uppercase tracking-widest">Video File</span>
            <input
              name="file"
              type="file"
              required
              accept="video/*"
              className="text-sm text-[#8e9285] file:mr-3 file:py-1.5 file:px-4 file:rounded-full file:border-0 file:text-xs file:font-medium file:bg-[#2a2a2a] file:text-[#e5e2e1] hover:file:bg-[#3a3a3a] cursor-pointer"
            />
          </label>

          <label className="flex flex-col gap-1">
            <span className="text-xs text-[#8e9285] uppercase tracking-widest">Thumbnail <span className="normal-case text-[#454545]">(optional)</span></span>
            <input
              name="thumbnail"
              type="file"
              accept="image/*"
              className="text-sm text-[#8e9285] file:mr-3 file:py-1.5 file:px-4 file:rounded-full file:border-0 file:text-xs file:font-medium file:bg-[#2a2a2a] file:text-[#e5e2e1] hover:file:bg-[#3a3a3a] cursor-pointer"
            />
          </label>

          <label className="flex items-center gap-2 cursor-pointer">
            <input
              name="transcode"
              type="checkbox"
              value="true"
              className="w-4 h-4 accent-[var(--color-accent)] rounded"
            />
            <span className="text-sm text-[#c4c8ba]">Start transcoding to HLS after upload</span>
          </label>

          {error && <p className="text-xs text-red-400">{error}</p>}

          <div className="flex gap-3 pt-1">
            <button
              type="submit"
              disabled={submitting}
              className="bg-[var(--color-accent)] hover:brightness-110 disabled:opacity-50 text-[#1b3706] font-bold px-6 py-2.5 rounded-full text-sm transition-all active:scale-95"
            >
              {submitting ? 'Uploading...' : 'Upload Episode'}
            </button>
            <button
              type="button"
              onClick={() => { setOpen(false); setError('') }}
              className="border border-[#43483d] text-[#8e9285] hover:text-[#e5e2e1] font-medium px-6 py-2.5 rounded-full text-sm transition-all"
            >
              Cancel
            </button>
          </div>
        </form>
      )}
    </div>
  )
}

function EpisodesSection({ id }: { id: string }) {
  const [episodes, setEpisodes] = useState<Episode[]>([])
  const [activeSeason, setActiveSeason] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)

  const fetchEpisodes = () => {
    setLoading(true)
    fetch(`/api/titles/${id}/episodes`)
      .then((r) => r.ok ? r.json() : [])
      .then((data: Episode[]) => {
        setEpisodes(data)
        if (activeSeason === null && data.length > 0) {
          const seasons = [...new Set(data.map((e) => e.season))].sort((a, b) => a - b)
          setActiveSeason(seasons[0])
        }
      })
      .catch(() => setEpisodes([]))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    fetchEpisodes()
  }, [id]) // eslint-disable-line react-hooks/exhaustive-deps

  const seasons = [...new Set(episodes.map((e) => e.season))].sort((a, b) => a - b)
  const currentSeason = activeSeason ?? seasons[0] ?? 1
  const filtered = episodes.filter((e) => e.season === currentSeason).sort((a, b) => a.number - b.number)

  return (
    <section className="mt-10">
      <h2 className="font-[family-name:var(--font-manrope)] text-xl font-bold text-[#e5e2e1] mb-4">Episodes</h2>

      {/* Season tabs */}
      {seasons.length > 1 && (
        <div className="flex gap-2 mb-5 flex-wrap">
          {seasons.map((s) => (
            <button
              key={s}
              onClick={() => setActiveSeason(s)}
              className={`px-4 py-1.5 rounded-full text-sm font-medium transition-all ${
                s === currentSeason
                  ? 'bg-[var(--color-accent)] text-[#1b3706]'
                  : 'bg-[#1c1b1b] text-[#8e9285] hover:text-[#e5e2e1] border border-[#2a2a2a] hover:border-[#3a3a3a]'
              }`}
            >
              Season {s}
            </button>
          ))}
        </div>
      )}

      {/* Episode list */}
      <div className="flex flex-col gap-2">
        {loading ? (
          <p className="text-sm text-[#454545]">Loading episodes...</p>
        ) : filtered.length === 0 ? (
          <p className="text-sm text-[#454545]">No episodes yet for this season.</p>
        ) : (
          filtered.map((ep) => <EpisodeItem key={ep.id} ep={ep} />)
        )}
      </div>

      <AddEpisodeForm id={id} onAdded={fetchEpisodes} />
    </section>
  )
}

export default function TitlePageClient({ id: staticId }: { id: string }) {
  const pathname = usePathname()
  const parts = pathname.split('/').filter(Boolean)
  const id = parts[parts.indexOf('title') + 1] ?? staticId

  const [data, setData] = useState<TitleData | null>(null)

  useEffect(() => {
    fetch(`/api/titles/${id}`)
      .then((r) => r.ok ? r.json() : null)
      .then((d) => setData(d))
      .catch(() => {})
  }, [id])

  const title = data?.title ?? `Title ${id}`
  const year = data?.year ?? ''
  const rating = data?.rating ?? ''
  const genres = data?.genre ?? []
  const synopsis = data?.synopsis ?? ''
  const director = data?.director ?? ''
  const isSeries = data?.type === 'series'

  return (
    <div className="min-h-screen bg-[#131313]">
      <NavBar />

      {/* Backdrop hero */}
      <div className="relative h-[60vh] bg-[#0e0e0e] overflow-hidden">
        <img
          key={`backdrop-${id}`}
          src={`/api/titles/${id}/thumbnail/backdrop`}
          alt=""
          className="absolute inset-0 w-full h-full object-cover opacity-60"
          onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
        />
        <div className="absolute inset-0 bg-gradient-to-r from-[#131313]/90 via-[#131313]/40 to-transparent" />
        <div className="absolute inset-0 bg-gradient-to-t from-[#131313] via-transparent to-transparent" />
      </div>

      <main className="max-w-screen-2xl mx-auto px-8 -mt-48 relative z-10 pb-16">
        <div className="flex gap-10">
          {/* Poster */}
          <div className="flex-shrink-0 w-44 aspect-[2/3] bg-[#1c1b1b] rounded-xl overflow-hidden shadow-2xl shadow-black/60 ring-1 ring-white/5">
            <img
              key={`poster-${id}`}
              src={`/api/titles/${id}/thumbnail/poster`}
              alt=""
              className="w-full h-full object-cover"
              onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
            />
          </div>

          {/* Info */}
          <div className="flex-1 pt-4">
            <h1 className="font-[family-name:var(--font-manrope)] text-5xl font-black text-[#e5e2e1] tracking-tight mb-3">
              {title}
            </h1>
            <div className="flex items-center gap-3 text-sm text-[#8e9285] mb-4">
              {year && <><span>{year}</span><span>·</span></>}
              {rating && <><span>{rating}</span><span>·</span></>}
              {genres.map((g) => (
                <span key={g} className="px-2 py-0.5 rounded bg-[#1c1b1b] text-xs">{g}</span>
              ))}
            </div>
            {synopsis && (
              <p className="text-[#c4c8ba] max-w-xl leading-relaxed mb-6">{synopsis}</p>
            )}
            <div className="flex gap-4 mb-8">
              {!isSeries && (
                <Link
                  href={`/watch/${id}`}
                  className="bg-[var(--color-accent)] hover:brightness-110 text-[#1b3706] font-bold px-8 py-3 rounded-full text-sm transition-all active:scale-95 flex items-center gap-2"
                >
                  ▶ Play
                </Link>
              )}
              <button className="border border-[#43483d] hover:border-[var(--color-accent)]/50 text-[#e5e2e1] font-medium px-8 py-3 rounded-full text-sm transition-all active:scale-95">
                + My List
              </button>
              {!isSeries && <TranscodeButton titleId={id} />}
              <Link
                href={`/admin/titles/${id}/edit`}
                className="border border-[#43483d] hover:border-[var(--color-accent)]/50 text-[#8e9285] hover:text-[#e5e2e1] font-medium px-5 py-3 rounded-full text-sm transition-all active:scale-95 flex items-center gap-1.5"
              >
                <svg xmlns="http://www.w3.org/2000/svg" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                Edit
              </Link>
            </div>
            {director && (
              <>
                <p className="text-xs text-[#8e9285] uppercase tracking-widest mb-1">Director</p>
                <p className="text-sm text-[#c4c8ba] mb-4">{director}</p>
              </>
            )}
          </div>
        </div>

        {/* Episodes section for series */}
        {isSeries && <EpisodesSection id={id} />}
      </main>
    </div>
  )
}
