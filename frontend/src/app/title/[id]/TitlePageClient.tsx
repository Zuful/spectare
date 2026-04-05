'use client'
import { useEffect, useState } from 'react'
import { usePathname } from 'next/navigation'
import Link from 'next/link'
import NavBar from '@/components/NavBar'
import TranscodeButton from '@/components/TranscodeButton'

type TitleData = {
  id: string; title: string; year: number; genre: string[]
  type: string; rating: string; synopsis: string; director: string
  streamReady: boolean; directPath?: string
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

  return (
    <div className="min-h-screen bg-[#131313]">
      <NavBar />

      {/* Backdrop hero */}
      <div className="relative h-[60vh] bg-[#0e0e0e] overflow-hidden">
        <img
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
              <Link
                href={`/watch/${id}`}
                className="bg-[var(--color-accent)] hover:brightness-110 text-[#1b3706] font-bold px-8 py-3 rounded-full text-sm transition-all active:scale-95 flex items-center gap-2"
              >
                ▶ Play
              </Link>
              <button className="border border-[#43483d] hover:border-[var(--color-accent)]/50 text-[#e5e2e1] font-medium px-8 py-3 rounded-full text-sm transition-all active:scale-95">
                + My List
              </button>
              <TranscodeButton titleId={id} />
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
      </main>
    </div>
  )
}
