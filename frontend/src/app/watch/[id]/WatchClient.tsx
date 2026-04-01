'use client'
import Link from 'next/link'

export default function WatchClient({ id }: { id: string }) {
  return (
    <div className="h-screen bg-black flex flex-col overflow-hidden">
      {/* Breadcrumb */}
      <div className="flex items-center gap-3 px-6 py-3 z-10">
        <Link href={`/title/${id}`} className="flex items-center gap-2 text-[#8e9285] hover:text-[#e5e2e1] transition-colors text-sm">
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="m15 18-6-6 6-6"/>
          </svg>
          Back to Title {id}
        </Link>
      </div>

      {/* Video area */}
      <div className="flex-1 bg-[#0a0a0a] flex items-center justify-center relative">
        <div className="text-center text-[#353534]">
          <div className="text-6xl mb-4">▶</div>
          <p className="text-sm font-medium">Stream not yet available</p>
          <p className="text-xs mt-1 text-[#2a2a2a]">HLS transcoding coming soon</p>
        </div>

        {/* Player controls overlay */}
        <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/80 to-transparent px-6 py-4">
          <div className="w-full h-1 bg-[#2a2a2a] rounded-full overflow-hidden mb-3 cursor-pointer">
            <div className="h-full bg-[#87a96b] w-0 transition-all" />
          </div>
          <div className="flex items-center gap-4 text-[#e5e2e1]">
            <button className="hover:text-[#87a96b] transition-colors">▶</button>
            <span className="text-xs text-[#8e9285] font-mono">0:00 / 0:00</span>
            <div className="flex-1" />
            <button className="text-xs text-[#8e9285] hover:text-[#e5e2e1] transition-colors">CC</button>
            <button className="text-xs text-[#8e9285] hover:text-[#e5e2e1] transition-colors">HD</button>
            <button className="text-[#8e9285] hover:text-[#e5e2e1] transition-colors" aria-label="Fullscreen">⛶</button>
          </div>
        </div>
      </div>

      {/* Keyboard hints */}
      <div className="px-6 py-1 flex gap-6 text-[10px] text-[#353534] font-mono bg-[#0a0a0a]">
        {[['SPACE', 'Play/Pause'], ['F', 'Fullscreen'], ['M', 'Mute'], ['←→', 'Seek 10s']].map(([key, desc]) => (
          <span key={key}><span className="text-[#454652]">{key}</span> {desc}</span>
        ))}
      </div>

      {/* Multi-tab bar */}
      <div className="bg-[#0e0e0e] border-t border-[#1c1b1b] flex items-center gap-1 px-2 h-14">
        <div className="flex items-center gap-2 px-3 py-1.5 bg-[#1c1b1b] rounded text-sm border-b-2 border-[#87a96b] min-w-0">
          <div className="w-10 aspect-video bg-[#2a2a2a] rounded flex-shrink-0" />
          <span className="text-[#e5e2e1] font-medium truncate text-xs">Title {id}</span>
          <button className="text-[#8e9285] hover:text-[#e5e2e1] ml-1 flex-shrink-0 text-xs">×</button>
        </div>
        <button className="flex items-center justify-center w-8 h-8 text-[#8e9285] hover:text-[#e5e2e1] hover:bg-[#1c1b1b] rounded transition-colors ml-1 text-lg">
          +
        </button>
      </div>
    </div>
  )
}
