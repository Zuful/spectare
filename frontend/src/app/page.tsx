'use client'
import { useEffect, useState } from 'react'
import NavBar from '@/components/NavBar'
import Link from 'next/link'
import { getAllInProgress } from '@/lib/watchProgress'

type TitleData = {
  id: string; title: string; year: number; genre: string[]
  type: string; rating: string; synopsis: string; director: string
  streamReady: boolean
}

type ContinueItem = { id: string; title: string; currentTime: number; duration: number; href: string }

export default function HomePage() {
  const [titles, setTitles] = useState<TitleData[]>([])
  const [continueItems, setContinueItems] = useState<ContinueItem[]>([])

  useEffect(() => {
    fetch('/api/titles')
      .then((r) => r.ok ? r.json() : [])
      .then((d) => setTitles(Array.isArray(d) ? d : []))
      .catch(() => {})
  }, [])

  useEffect(() => {
    const inProgress = getAllInProgress().slice(0, 8)
    if (inProgress.length === 0) return

    Promise.all(
      inProgress.map(async (entry) => {
        // Try titles first, then episodes
        let res = await fetch(`/api/titles/${entry.id}`)
        let isEpisode = false
        if (!res.ok) {
          res = await fetch(`/api/episodes/${entry.id}`)
          isEpisode = true
        }
        if (!res.ok) return null
        const data = await res.json()
        const title = data.title ?? `ID ${entry.id}`
        const href = isEpisode ? `/watch/episode/${entry.id}` : `/watch/${entry.id}`
        return { id: entry.id, title, currentTime: entry.currentTime, duration: entry.duration, href } as ContinueItem
      })
    ).then((results) => {
      setContinueItems(results.filter((r): r is ContinueItem => r !== null))
    }).catch(() => {})
  }, [])

  const featured = titles[0] ?? null
  const movies = titles.filter((t) => t.type === 'movie')
  const series = titles.filter((t) => t.type === 'series' || t.type === 'show')
  const recent = titles.slice(0, 12)

  return (
    <div className="min-h-screen bg-[#131313]">
      <NavBar />

      {/* Hero */}
      {featured ? (
        <section className="relative h-[85vh] flex items-end overflow-hidden">
          <img
            src={`/api/titles/${featured.id}/thumbnail/backdrop`}
            alt=""
            className="absolute inset-0 w-full h-full object-cover opacity-50"
            onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
          />
          <div className="absolute inset-0 bg-gradient-to-r from-[#131313]/90 via-[#131313]/40 to-transparent" />
          <div className="absolute inset-0 bg-gradient-to-t from-[#131313] via-transparent to-transparent" />
          <div className="relative z-10 max-w-screen-2xl mx-auto px-8 pb-20 w-full">
            <p className="text-[10px] font-bold tracking-[0.2em] uppercase text-[var(--color-accent)] mb-3">Featured</p>
            <h1 className="font-[family-name:var(--font-manrope)] text-5xl font-black text-[#e5e2e1] tracking-tight leading-tight mb-4 max-w-xl">
              {featured.title}
            </h1>
            {featured.synopsis && (
              <p className="text-[#c4c8ba] text-base max-w-md mb-6 leading-relaxed">
                {featured.synopsis}
              </p>
            )}
            <div className="flex items-center gap-4">
              <Link
                href={`/watch/${featured.id}`}
                className="bg-[var(--color-accent)] hover:brightness-110 text-[#1b3706] font-bold px-8 py-3 rounded-full text-sm transition-all active:scale-95 flex items-center gap-2"
              >
                ▶ Play
              </Link>
              <Link
                href={`/title/${featured.id}`}
                className="bg-white/10 hover:bg-white/20 text-[#e5e2e1] font-medium px-8 py-3 rounded-full text-sm transition-all active:scale-95 backdrop-blur-sm"
              >
                More Info
              </Link>
            </div>
          </div>
        </section>
      ) : (
        <section className="relative h-[85vh] flex items-center justify-center bg-gradient-to-b from-[#0e0e0e] to-[#131313]">
          <div className="text-center">
            <p className="text-[#43483d] text-xl mb-2">No titles yet</p>
            <p className="text-[#43483d] text-sm">Add a video to get started</p>
          </div>
        </section>
      )}

      {/* Content rows */}
      <section className="max-w-screen-2xl mx-auto px-8 pb-32 space-y-12">
        {continueItems.length > 0 && (
          <div>
            <h2 className="font-[family-name:var(--font-manrope)] text-xl font-bold text-[#e5e2e1] mb-4">Continue Watching</h2>
            <div className="flex gap-4 overflow-x-auto pb-2">
              {continueItems.map((item) => (
                <ContinueCard key={item.id} item={item} />
              ))}
            </div>
          </div>
        )}
        {recent.length > 0 && (
          <TitleRow label="Recently Added" items={recent} />
        )}
        {movies.length > 0 && (
          <TitleRow label="Movies" items={movies} />
        )}
        {series.length > 0 && (
          <TitleRow label="Series" items={series} />
        )}
      </section>
    </div>
  )
}

function ContinueCard({ item }: { item: ContinueItem }) {
  const pct = item.duration > 0 ? (item.currentTime / item.duration) * 100 : 0
  return (
    <Link
      href={item.href}
      className="flex-shrink-0 w-64 aspect-video bg-[#1c1b1b] rounded-lg overflow-hidden group relative hover:scale-105 transition-transform duration-300"
    >
      <img
        src={`/api/titles/${item.id}/thumbnail/card`}
        alt=""
        className="w-full h-full object-cover"
        onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
      />
      <div className="absolute inset-0 bg-gradient-to-t from-black/70 to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
      <div className="absolute bottom-0 left-0 right-0">
        <div className="h-0.5 bg-white/20">
          <div className="h-full bg-[var(--color-accent)]" style={{ width: `${pct}%` }} />
        </div>
      </div>
      <div className="absolute bottom-2 left-3 opacity-0 group-hover:opacity-100 transition-opacity">
        <p className="text-xs font-bold text-[#e5e2e1]">{item.title}</p>
      </div>
    </Link>
  )
}

function TitleRow({ label, items }: { label: string; items: TitleData[] }) {
  return (
    <div>
      <h2 className="font-[family-name:var(--font-manrope)] text-xl font-bold text-[#e5e2e1] mb-4">{label}</h2>
      <div className="flex gap-4 overflow-x-auto pb-2">
        {items.map((t) => (
          <Link
            key={t.id}
            href={`/title/${t.id}`}
            className="flex-shrink-0 w-64 aspect-video bg-[#1c1b1b] rounded-lg overflow-hidden group relative hover:scale-105 transition-transform duration-300"
          >
            <img
              src={`/api/titles/${t.id}/thumbnail/card`}
              alt=""
              className="w-full h-full object-cover"
              onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
            />
            <div className="absolute inset-0 bg-gradient-to-t from-black/70 to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
            <div className="absolute bottom-2 left-3 opacity-0 group-hover:opacity-100 transition-opacity">
              <p className="text-xs font-bold text-[#e5e2e1]">{t.title}</p>
              {t.year > 0 && <p className="text-[10px] text-[#8e9285]">{t.year}</p>}
            </div>
          </Link>
        ))}
      </div>
    </div>
  )
}
