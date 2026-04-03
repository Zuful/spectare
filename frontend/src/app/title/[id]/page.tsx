import NavBar from '@/components/NavBar'
import TranscodeButton from '@/components/TranscodeButton'
import Link from 'next/link'

export function generateStaticParams() {
  return Array.from({ length: 20 }, (_, i) => ({ id: String(i + 1) }))
}

export default async function TitlePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params

  return (
    <div className="min-h-screen bg-[#131313]">
      <NavBar />

      {/* Backdrop hero */}
      <div className="relative h-[60vh] bg-[#0e0e0e] overflow-hidden">
        {/* Backdrop image — hides gracefully if absent */}
        <img
          src={`/api/titles/${id}/thumbnail/backdrop`}
          alt=""
          className="absolute inset-0 w-full h-full object-cover opacity-60"
          onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
        />
        {/* Gradient overlays */}
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
              Title {id}
            </h1>
            <div className="flex items-center gap-3 text-sm text-[#8e9285] mb-4">
              <span>2024</span>
              <span>·</span>
              <span>TV-MA</span>
              <span>·</span>
              <span>2h 15m</span>
              <span className="px-2 py-0.5 rounded bg-[#1c1b1b] text-xs">Drama</span>
              <span className="px-2 py-0.5 rounded bg-[#1c1b1b] text-xs">Sci-Fi</span>
            </div>
            <p className="text-[#c4c8ba] max-w-xl leading-relaxed mb-6">
              A story of courage, betrayal, and redemption set against the backdrop of a fractured world.
            </p>
            <div className="flex gap-4 mb-8">
              <Link
                href={`/watch/${id}`}
                className="bg-[#87a96b] hover:brightness-110 text-[#1b3706] font-bold px-8 py-3 rounded-full text-sm transition-all active:scale-95 flex items-center gap-2"
              >
                ▶ Play
              </Link>
              <button className="border border-[#43483d] hover:border-[#87a96b]/50 text-[#e5e2e1] font-medium px-8 py-3 rounded-full text-sm transition-all active:scale-95">
                + My List
              </button>
              <TranscodeButton titleId={id} />
              <Link
                href={`/admin/titles/${id}/edit`}
                className="border border-[#43483d] hover:border-[#87a96b]/50 text-[#8e9285] hover:text-[#e5e2e1] font-medium px-5 py-3 rounded-full text-sm transition-all active:scale-95 flex items-center gap-1.5"
              >
                <svg xmlns="http://www.w3.org/2000/svg" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                Edit
              </Link>
            </div>
            <p className="text-xs text-[#8e9285] uppercase tracking-widest mb-1">Director</p>
            <p className="text-sm text-[#c4c8ba] mb-4">Christopher Nolan</p>
          </div>
        </div>

        {/* More like this */}
        <div className="mt-16">
          <h2 className="font-[family-name:var(--font-manrope)] text-xl font-bold text-[#e5e2e1] mb-4">More Like This</h2>
          <div className="flex gap-4 overflow-x-auto pb-2">
            {Array.from({ length: 6 }).map((_, i) => (
              <Link key={i} href={`/title/${i + 10}`} className="flex-shrink-0 w-52 aspect-video bg-[#1c1b1b] rounded-lg hover:scale-105 transition-transform duration-300" />
            ))}
          </div>
        </div>
      </main>
    </div>
  )
}
