'use client'
import { useEffect, useState, useMemo } from 'react'
import Link from 'next/link'
import NavBar from '@/components/NavBar'

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

export default function BrowsePage() {
  const [titles, setTitles] = useState<Title[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [activeGenre, setActiveGenre] = useState('All')
  const [activeType, setActiveType] = useState<'all' | 'movie' | 'series'>('all')

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

  return (
    <div className="min-h-screen bg-[#131313]">
      <NavBar />
      <main className="max-w-screen-2xl mx-auto px-8 pt-28 pb-16">
        <h1 className="font-[family-name:var(--font-manrope)] text-4xl font-black text-[#e5e2e1] mb-8">Browse</h1>

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
            <button
              key={t}
              onClick={() => setActiveType(t)}
              className={`px-4 py-1.5 rounded-full text-xs font-bold transition-all ${
                activeType === t ? 'bg-[#87a96b]/20 text-[#87a96b] border border-[#87a96b]/40' : 'bg-[#1c1b1b] text-[#8e9285] hover:text-[#c4c8ba]'
              }`}
            >
              {t === 'all' ? 'All types' : t === 'movie' ? 'Movies' : 'Series'}
            </button>
          ))}
        </div>

        {/* Genre filters — only shown when there are genres */}
        {genres.length > 1 && (
          <div className="flex gap-2 flex-wrap mb-8">
            {genres.map((g) => (
              <button
                key={g}
                onClick={() => setActiveGenre(g)}
                className={`px-4 py-1.5 rounded-full text-xs font-bold transition-all ${
                  activeGenre === g ? 'bg-[#87a96b] text-[#1b3706]' : 'bg-[#1c1b1b] text-[#c4c8ba] hover:bg-[#2a2a2a]'
                }`}
              >
                {g}
              </button>
            ))}
          </div>
        )}

        {/* Results count */}
        {!loading && (
          <p className="text-xs text-[#454545] mb-4">
            {filtered.length} {filtered.length === 1 ? 'title' : 'titles'}
            {(activeGenre !== 'All' || activeType !== 'all' || search) && ' matching filters'}
          </p>
        )}

        {/* Grid */}
        {loading ? (
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-4">
            {Array.from({ length: 10 }).map((_, i) => (
              <div key={i} className="aspect-video bg-[#1c1b1b] rounded-lg animate-pulse" />
            ))}
          </div>
        ) : filtered.length === 0 ? (
          <div className="text-center py-24 text-[#454545]">
            <p className="text-4xl mb-3">◻</p>
            <p className="text-sm">No titles found</p>
            {titles.length === 0 && (
              <Link href="/admin/upload" className="inline-block mt-4 text-[#87a96b] text-xs hover:underline">
                Add your first title →
              </Link>
            )}
          </div>
        ) : (
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-4">
            {filtered.map((t) => (
              <Link
                key={t.id}
                href={`/title/${t.id}`}
                className="group relative aspect-video bg-[#1c1b1b] rounded-lg overflow-hidden hover:scale-105 transition-transform duration-300 hover:ring-1 hover:ring-[#87a96b]/30"
              >
                {/* Gradient overlay */}
                <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-transparent" />

                {/* Rating badge */}
                {t.rating && (
                  <span className="absolute top-2 right-2 text-[10px] font-bold bg-black/60 text-[#8e9285] px-1.5 py-0.5 rounded backdrop-blur-sm">
                    {t.rating}
                  </span>
                )}

                {/* Stream indicator */}
                {hasStream(t) && (
                  <span className="absolute top-2 left-2 text-[9px] font-mono font-bold bg-[#87a96b]/20 text-[#87a96b] px-1.5 py-0.5 rounded">
                    {t.streamReady ? 'HLS' : 'DIRECT'}
                  </span>
                )}

                {/* Info */}
                <div className="absolute bottom-0 left-0 right-0 p-2.5">
                  <p className="text-xs font-bold text-[#e5e2e1] truncate leading-tight">{t.title}</p>
                  <div className="flex items-center gap-1.5 mt-0.5 flex-wrap">
                    {t.year > 0 && <span className="text-[10px] text-[#8e9285]">{t.year}</span>}
                    {t.genre?.slice(0, 2).map((g) => (
                      <span key={g} className="text-[9px] text-[#87a96b] bg-[#87a96b]/10 px-1 py-0.5 rounded">
                        {g}
                      </span>
                    ))}
                  </div>
                </div>
              </Link>
            ))}
          </div>
        )}
      </main>
    </div>
  )
}
