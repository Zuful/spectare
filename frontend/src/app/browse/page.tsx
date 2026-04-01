import NavBar from '@/components/NavBar'

const GENRES = ['All', 'Action', 'Drama', 'Sci-Fi', 'Comedy', 'Documentary', 'Thriller']

export default function BrowsePage() {
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
            placeholder="Search titles, genres, people…"
            className="bg-transparent outline-none flex-1 text-sm text-[#e5e2e1] placeholder:text-[#8e9285]"
          />
        </div>

        {/* Genre filters */}
        <div className="flex gap-2 flex-wrap mb-8">
          {GENRES.map((g) => (
            <button
              key={g}
              className={`px-4 py-1.5 rounded-full text-xs font-bold transition-all ${
                g === 'All'
                  ? 'bg-[#87a96b] text-[#1b3706]'
                  : 'bg-[#1c1b1b] text-[#c4c8ba] hover:bg-[#2a2a2a]'
              }`}
            >
              {g}
            </button>
          ))}
        </div>

        {/* Grid */}
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-4">
          {Array.from({ length: 20 }).map((_, i) => (
            <div key={i} className="aspect-video bg-[#1c1b1b] rounded-lg hover:scale-105 transition-transform duration-300 cursor-pointer group relative overflow-hidden">
              <div className="absolute inset-0 bg-gradient-to-t from-black/70 to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
              <span className="absolute top-2 right-2 text-[10px] font-bold bg-[#87a96b]/20 text-[#87a96b] px-1.5 py-0.5 rounded">
                {i % 3 === 0 ? 'PG-13' : i % 3 === 1 ? 'R' : 'TV-MA'}
              </span>
            </div>
          ))}
        </div>
      </main>
    </div>
  )
}
