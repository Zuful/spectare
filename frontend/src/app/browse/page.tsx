'use client'
import { useEffect, useState, useMemo, useRef, useCallback } from 'react'
import Link from 'next/link'
import NavBar from '@/components/NavBar'
import { getProgress, isWatched } from '@/lib/watchProgress'

type Title = {
  id: string
  title: string
  year: number
  genre: string[]
  type: 'movie' | 'series'
  rating: string
  synopsis: string
  director: string
  streamReady: boolean
  directPath?: string
}

function hasStream(t: Title) {
  return t.streamReady || !!t.directPath
}

// ── Hover-preview card ────────────────────────────────────────────────────────

type CardProps = { title: Title; layout: 'landscape' | 'portrait' }

function TitleCard({ title: t, layout }: CardProps) {
  const [hovered, setHovered] = useState(false)
  const [expanded, setExpanded] = useState(false)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const videoRef = useRef<HTMLVideoElement>(null)
  const [pct, setPct] = useState(0)
  const [watched, setWatched] = useState(false)

  useEffect(() => {
    const p = getProgress(t.id)
    if (p && p.duration > 0) setPct((p.currentTime / p.duration) * 100)
    setWatched(isWatched(t.id))
  }, [t.id])

  const thumbVariant = layout === 'portrait' ? 'poster' : 'card'
  const aspectClass = layout === 'portrait' ? 'aspect-[2/3]' : 'aspect-video'

  const handleMouseEnter = useCallback(() => {
    setHovered(true)
    timerRef.current = setTimeout(() => {
      setExpanded(true)
      videoRef.current?.play().catch(() => {})
    }, 1500)
  }, [])

  const handleMouseLeave = useCallback(() => {
    setHovered(false)
    setExpanded(false)
    if (timerRef.current) clearTimeout(timerRef.current)
    if (videoRef.current) {
      videoRef.current.pause()
      videoRef.current.currentTime = 0
    }
  }, [])

  return (
    <div className="relative group" onMouseEnter={handleMouseEnter} onMouseLeave={handleMouseLeave}>
      {/* Base card */}
      <Link
        href={`/title/${t.id}`}
        className={`block relative ${aspectClass} bg-[#1c1b1b] rounded-lg overflow-hidden transition-transform duration-300 ${
          hovered && !expanded ? 'scale-105' : ''
        } hover:ring-1 hover:ring-[var(--color-accent)]/30`}
      >
        <img
          src={`/api/titles/${t.id}/thumbnail/${thumbVariant}`}
          alt=""
          className="absolute inset-0 w-full h-full object-cover"
          onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
        />
        <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-transparent" />
        {t.rating && (
          <span className="absolute top-2 right-2 text-[10px] font-bold bg-black/60 text-[#8e9285] px-1.5 py-0.5 rounded backdrop-blur-sm">
            {t.rating}
          </span>
        )}
        {hasStream(t) && (
          <span className="absolute top-2 left-2 text-[9px] font-mono font-bold bg-[var(--color-accent)]/20 text-[var(--color-accent)] px-1.5 py-0.5 rounded">
            {t.streamReady ? 'HLS' : 'DIRECT'}
          </span>
        )}
        <div className="absolute bottom-0 left-0 right-0 p-2.5">
          <p className="text-xs font-bold text-[#e5e2e1] truncate leading-tight">{t.title}</p>
          <div className="flex items-center gap-1.5 mt-0.5 flex-wrap">
            {t.year > 0 && <span className="text-[10px] text-[#8e9285]">{t.year}</span>}
            {t.genre?.slice(0, 2).map((g) => (
              <span key={g} className="text-[9px] text-[var(--color-accent)] bg-[var(--color-accent)]/10 px-1 py-0.5 rounded">{g}</span>
            ))}
          </div>
        </div>
        {pct > 0 && (
          <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-white/20">
            <div className="h-full bg-[var(--color-accent)]" style={{ width: `${pct}%` }} />
          </div>
        )}
        {watched && (
          <div className="absolute top-2 right-2 w-5 h-5 rounded-full bg-[var(--color-accent)] flex items-center justify-center">
            <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3"><polyline points="20 6 9 17 4 12"/></svg>
          </div>
        )}
      </Link>

      {/* Expanded hover card with preview */}
      {expanded && (
        <div className={`absolute ${layout === 'portrait' ? 'top-0 left-0 w-[180%]' : 'top-0 left-0 right-0'} z-30 bg-[#1c1b1b] rounded-xl overflow-hidden shadow-2xl shadow-black/60 ring-1 ring-[var(--color-accent)]/20`}
          style={{ minWidth: layout === 'portrait' ? undefined : '100%' }}>
          {/* Video preview */}
          <div className="relative aspect-video bg-black">
            <video
              ref={videoRef}
              src={`/api/titles/${t.id}/preview`}
              className="w-full h-full object-cover"
              muted
              loop
              playsInline
              preload="none"
            />
            <div className="absolute inset-0 bg-gradient-to-t from-[#1c1b1b] via-transparent to-transparent" />
          </div>
          {/* Info */}
          <div className="px-3 pb-3 -mt-4 relative z-10">
            <p className="text-sm font-bold text-[#e5e2e1] truncate">{t.title}</p>
            <div className="flex items-center gap-2 mt-1 mb-2">
              {t.year > 0 && <span className="text-[10px] text-[#8e9285]">{t.year}</span>}
              {t.rating && <span className="text-[10px] text-[#8e9285] border border-[#43483d] px-1 rounded">{t.rating}</span>}
              {t.genre?.slice(0, 3).map((g) => (
                <span key={g} className="text-[9px] text-[var(--color-accent)]">{g}</span>
              ))}
            </div>
            <div className="flex gap-2">
              <Link
                href={`/watch/${t.id}`}
                className="flex-1 bg-[var(--color-accent)] text-[#1b3706] font-bold text-xs py-1.5 rounded-full text-center hover:brightness-110 transition-all"
                onClick={(e) => e.stopPropagation()}
              >
                ▶ Play
              </Link>
              <Link
                href={`/title/${t.id}`}
                className="px-3 border border-[#43483d] text-[#e5e2e1] text-xs py-1.5 rounded-full hover:border-[var(--color-accent)]/50 transition-all"
                onClick={(e) => e.stopPropagation()}
              >
                Info
              </Link>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

// ── Page ──────────────────────────────────────────────────────────────────────

export default function BrowsePage() {
  const [titles, setTitles] = useState<Title[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [activeGenre, setActiveGenre] = useState('All')
  const [activeType, setActiveType] = useState<'all' | 'movie' | 'series'>('all')
  const [layout, setLayout] = useState<'landscape' | 'portrait'>('landscape')

  useEffect(() => {
    fetch('/api/titles')
      .then((r) => r.json())
      .then((data) => setTitles(Array.isArray(data) ? data : []))
      .catch(() => setTitles([]))
      .finally(() => setLoading(false))
  }, [])

  const genres = useMemo(() => {
    const set = new Set<string>()
    titles.forEach((t) => t.genre?.forEach((g) => set.add(g)))
    return ['All', ...Array.from(set).sort()]
  }, [titles])

  const filtered = useMemo(() => {
    const q = search.toLowerCase()
    return titles.filter((t) => {
      if (activeType !== 'all' && t.type !== activeType) return false
      if (activeGenre !== 'All' && !t.genre?.includes(activeGenre)) return false
      if (q) {
        const haystack = [t.title, t.director, ...(t.genre ?? [])].join(' ').toLowerCase()
        if (!haystack.includes(q)) return false
      }
      return true
    })
  }, [titles, search, activeGenre, activeType])

  const gridClass = layout === 'portrait'
    ? 'grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-7 gap-3'
    : 'grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-4'

  return (
    <div className="min-h-screen bg-[#131313]">
      <NavBar />
      <main className="max-w-screen-2xl mx-auto px-8 pt-28 pb-16">
        <div className="flex items-center justify-between mb-8">
          <h1 className="font-[family-name:var(--font-manrope)] text-4xl font-black text-[#e5e2e1]">Browse</h1>
          {/* Layout toggle */}
          <div className="flex items-center gap-1 bg-[#1c1b1b] rounded-lg p-1">
            <button
              onClick={() => setLayout('landscape')}
              title="Landscape"
              className={`p-1.5 rounded transition-colors ${layout === 'landscape' ? 'bg-[#2a2a2a] text-[#e5e2e1]' : 'text-[#454545] hover:text-[#8e9285]'}`}
            >
              <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
                <rect x="1" y="3" width="14" height="4" rx="1"/><rect x="1" y="9" width="14" height="4" rx="1"/>
              </svg>
            </button>
            <button
              onClick={() => setLayout('portrait')}
              title="Portrait"
              className={`p-1.5 rounded transition-colors ${layout === 'portrait' ? 'bg-[#2a2a2a] text-[#e5e2e1]' : 'text-[#454545] hover:text-[#8e9285]'}`}
            >
              <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
                <rect x="1" y="1" width="4" height="14" rx="1"/><rect x="6" y="1" width="4" height="14" rx="1"/><rect x="11" y="1" width="4" height="14" rx="1"/>
              </svg>
            </button>
          </div>
        </div>

        {/* Search */}
        <div className="flex items-center gap-3 bg-[#1c1b1b] rounded-full px-5 py-3 mb-6 max-w-xl">
          <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#8e9285" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/>
          </svg>
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search titles, genres, directors…"
            className="bg-transparent outline-none flex-1 text-sm text-[#e5e2e1] placeholder:text-[#8e9285]"
          />
          {search && (
            <button onClick={() => setSearch('')} className="text-[#8e9285] hover:text-[#e5e2e1] text-lg leading-none">×</button>
          )}
        </div>

        {/* Type toggle */}
        <div className="flex gap-2 mb-4">
          {(['all', 'movie', 'series'] as const).map((t) => (
            <button key={t} onClick={() => setActiveType(t)}
              className={`px-4 py-1.5 rounded-full text-xs font-bold transition-all ${
                activeType === t ? 'bg-[var(--color-accent)]/20 text-[var(--color-accent)] border border-[var(--color-accent)]/40' : 'bg-[#1c1b1b] text-[#8e9285] hover:text-[#c4c8ba]'
              }`}>
              {t === 'all' ? 'All types' : t === 'movie' ? 'Movies' : 'Series'}
            </button>
          ))}
        </div>

        {/* Genre filters */}
        {genres.length > 1 && (
          <div className="flex gap-2 flex-wrap mb-8">
            {genres.map((g) => (
              <button key={g} onClick={() => setActiveGenre(g)}
                className={`px-4 py-1.5 rounded-full text-xs font-bold transition-all ${
                  activeGenre === g ? 'bg-[var(--color-accent)] text-[#1b3706]' : 'bg-[#1c1b1b] text-[#c4c8ba] hover:bg-[#2a2a2a]'
                }`}>
                {g}
              </button>
            ))}
          </div>
        )}

        {!loading && (
          <p className="text-xs text-[#454545] mb-4">
            {filtered.length} {filtered.length === 1 ? 'title' : 'titles'}
            {(activeGenre !== 'All' || activeType !== 'all' || search) && ' matching filters'}
          </p>
        )}

        {/* Grid */}
        {loading ? (
          <div className={gridClass}>
            {Array.from({ length: 10 }).map((_, i) => (
              <div key={i} className={`${layout === 'portrait' ? 'aspect-[2/3]' : 'aspect-video'} bg-[#1c1b1b] rounded-lg animate-pulse`} />
            ))}
          </div>
        ) : filtered.length === 0 ? (
          <div className="text-center py-24 text-[#454545]">
            <p className="text-4xl mb-3">◻</p>
            <p className="text-sm">No titles found</p>
            {titles.length === 0 && (
              <Link href="/admin/upload" className="inline-block mt-4 text-[var(--color-accent)] text-xs hover:underline">
                Add your first title →
              </Link>
            )}
          </div>
        ) : (
          <div className={`${gridClass} relative`} style={{ zIndex: 0 }}>
            {filtered.map((t) => (
              <TitleCard key={t.id} title={t} layout={layout} />
            ))}
          </div>
        )}
      </main>
    </div>
  )
}
