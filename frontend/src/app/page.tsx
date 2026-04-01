import NavBar from '@/components/NavBar'
import Link from 'next/link'

export default function HomePage() {
  return (
    <div className="min-h-screen bg-[#131313]">
      <NavBar />

      {/* Hero */}
      <section className="relative h-[85vh] flex items-end bg-gradient-to-b from-[#0e0e0e] to-[#131313]">
        <div className="absolute inset-0 bg-gradient-to-r from-[#0e0e0e]/80 to-transparent" />
        <div className="relative z-10 max-w-screen-2xl mx-auto px-8 pb-20 w-full">
          <p className="text-[10px] font-bold tracking-[0.2em] uppercase text-[#87a96b] mb-3">Featured</p>
          <h1 className="font-[family-name:var(--font-manrope)] text-5xl font-black text-[#e5e2e1] tracking-tight leading-tight mb-4 max-w-xl">
            The Last Kingdom
          </h1>
          <p className="text-[#c4c8ba] text-base max-w-md mb-6 leading-relaxed">
            A displaced nobleman fights to reclaim his home in 9th century England while serving the king who conquered it.
          </p>
          <div className="flex items-center gap-4">
            <Link
              href="/watch/1"
              className="bg-[#87a96b] hover:brightness-110 text-[#1b3706] font-bold px-8 py-3 rounded-full text-sm transition-all active:scale-95 flex items-center gap-2"
            >
              ▶ Play
            </Link>
            <Link
              href="/title/1"
              className="bg-white/10 hover:bg-white/20 text-[#e5e2e1] font-medium px-8 py-3 rounded-full text-sm transition-all active:scale-95 backdrop-blur-sm"
            >
              More Info
            </Link>
          </div>
        </div>
      </section>

      {/* Content rows */}
      <section className="max-w-screen-2xl mx-auto px-8 pb-32 space-y-12">
        {['Continue Watching', 'Trending Now', 'New Releases'].map((rowTitle) => (
          <div key={rowTitle}>
            <h2 className="font-[family-name:var(--font-manrope)] text-xl font-bold text-[#e5e2e1] mb-4">{rowTitle}</h2>
            <div className="flex gap-4 overflow-x-auto pb-2">
              {Array.from({ length: 6 }).map((_, i) => (
                <Link
                  key={i}
                  href={`/title/${i + 1}`}
                  className="flex-shrink-0 w-64 aspect-video bg-[#1c1b1b] rounded-lg overflow-hidden group relative hover:scale-105 transition-transform duration-300"
                >
                  <div className="absolute inset-0 bg-gradient-to-t from-black/70 to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
                  <div className="absolute bottom-2 left-3 opacity-0 group-hover:opacity-100 transition-opacity">
                    <p className="text-xs font-bold text-[#e5e2e1]">Title {i + 1}</p>
                  </div>
                </Link>
              ))}
            </div>
          </div>
        ))}
      </section>
    </div>
  )
}
