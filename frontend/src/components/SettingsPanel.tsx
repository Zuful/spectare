'use client'
import { useState, useEffect } from 'react'
import { applyAccent, getStoredAccent, DEFAULT_ACCENT } from './ThemeProvider'

const PRESETS = [
  { name: 'Matcha',    color: '#87a96b' },
  { name: 'Sakura',   color: '#e07895' },
  { name: 'Ocean',    color: '#4a9eca' },
  { name: 'Sunset',   color: '#e07850' },
  { name: 'Lavender', color: '#9b7fe8' },
  { name: 'Gold',     color: '#c9a85c' },
  { name: 'Teal',     color: '#4db6ac' },
  { name: 'Crimson',  color: '#d94f4f' },
]

export default function SettingsPanel() {
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(DEFAULT_ACCENT)

  useEffect(() => {
    setActive(getStoredAccent())
  }, [])

  function handleSelect(color: string) {
    setActive(color)
    applyAccent(color)
  }

  return (
    <>
      {/* Floating trigger */}
      <button
        onClick={() => setOpen((v) => !v)}
        aria-label="Settings"
        className={`fixed bottom-6 right-6 z-50 w-10 h-10 rounded-full flex items-center justify-center transition-all shadow-lg
          ${open ? 'bg-[var(--color-accent)] text-[#1b3706]' : 'bg-[#1c1b1b] border border-[#2a2a2a] text-[#8e9285] hover:text-[#e5e2e1] hover:border-[#43483d]'}`}
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="12" cy="12" r="3"/>
          <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
        </svg>
      </button>

      {/* Panel */}
      {open && (
        <div className="fixed bottom-20 right-6 z-50 bg-[#1c1b1b] border border-[#2a2a2a] rounded-2xl p-4 shadow-2xl w-56">
          <p className="text-[10px] text-[#8e9285] uppercase tracking-widest mb-3">Accent colour</p>
          <div className="grid grid-cols-4 gap-2">
            {PRESETS.map(({ name, color }) => (
              <button
                key={color}
                title={name}
                onClick={() => handleSelect(color)}
                style={{ backgroundColor: color }}
                className={`w-full aspect-square rounded-lg transition-all active:scale-90 ${
                  active === color
                    ? 'ring-2 ring-offset-2 ring-offset-[#1c1b1b] ring-white/60 scale-110'
                    : 'hover:scale-105'
                }`}
              />
            ))}
          </div>
          <div className="mt-3 flex items-center gap-2">
            <label className="text-[10px] text-[#454545] uppercase tracking-widest flex-1">Custom</label>
            <input
              type="color"
              value={active}
              onChange={(e) => handleSelect(e.target.value)}
              className="w-8 h-6 rounded cursor-pointer border-0 bg-transparent p-0"
            />
          </div>
        </div>
      )}
    </>
  )
}
