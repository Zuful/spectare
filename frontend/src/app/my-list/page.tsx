import NavBar from '@/components/NavBar'

export default function MyListPage() {
  return (
    <div className="min-h-screen bg-[#131313]">
      <NavBar />
      <main className="max-w-screen-2xl mx-auto px-8 pt-28 pb-16">
        <div className="flex items-baseline gap-4 mb-8">
          <h1 className="font-[family-name:var(--font-manrope)] text-4xl font-black text-[#e5e2e1]">My List</h1>
          <span className="text-sm text-[#8e9285]">0 titles</span>
        </div>

        {/* Empty state */}
        <div className="flex flex-col items-center justify-center py-32 gap-4">
          <div className="w-16 h-16 rounded-full bg-[#87a96b]/10 flex items-center justify-center">
            <svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#87a96b" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16z"/>
            </svg>
          </div>
          <p className="text-[#e5e2e1] font-medium">Your list is empty</p>
          <p className="text-[#8e9285] text-sm">Add titles to watch them later</p>
          <a href="/browse" className="mt-2 bg-[#87a96b] hover:brightness-110 text-[#1b3706] font-bold px-6 py-2.5 rounded-full text-sm transition-all active:scale-95">
            Browse titles
          </a>
        </div>
      </main>
    </div>
  )
}
