'use client'
import Link from 'next/link'
import { usePathname } from 'next/navigation'

const links = [
  { href: '/', label: 'Home' },
  { href: '/browse', label: 'Browse' },
  { href: '/my-list', label: 'My List' },
]

const adminLinks = [
  { href: '/admin/upload', label: '+ Add title' },
]

export default function NavBar() {
  const pathname = usePathname()

  return (
    <header className="fixed top-0 w-full z-50 bg-[#131313]/80 backdrop-blur-xl border-b border-white/5">
      <div className="max-w-screen-2xl mx-auto px-8 h-16 flex items-center justify-between">
        <div className="flex items-center gap-10">
          <Link href="/" className="text-[#87a96b] font-[family-name:var(--font-manrope)] font-black text-xl tracking-tight uppercase">
            Spectare
          </Link>
          <nav className="flex items-center gap-6">
            {links.map((l) => (
              <Link
                key={l.href}
                href={l.href}
                className={`text-sm font-medium transition-colors ${
                  pathname === l.href ? 'text-[#e5e2e1]' : 'text-[#c4c8ba] hover:text-[#e5e2e1]'
                }`}
              >
                {l.label}
              </Link>
            ))}
          </nav>
        </div>
        <div className="flex items-center gap-4">
          {adminLinks.map((l) => (
            <Link
              key={l.href}
              href={l.href}
              className="text-xs font-medium text-[#8e9285] hover:text-[#87a96b] transition-colors border border-[#2a2a2a] hover:border-[#87a96b]/40 px-3 py-1.5 rounded-full"
            >
              {l.label}
            </Link>
          ))}
          <button className="text-[#c4c8ba] hover:text-[#e5e2e1] transition-colors" aria-label="Search">
            <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/>
            </svg>
          </button>
          <div className="w-8 h-8 rounded-full bg-[#87a96b]/20 border border-[#87a96b]/40 flex items-center justify-center text-xs font-bold text-[#87a96b]">
            U
          </div>
        </div>
      </div>
    </header>
  )
}
