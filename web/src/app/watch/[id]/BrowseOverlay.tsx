'use client'
import { useEffect, useState } from 'react'

type Title = { id: string; title: string; type: string }

interface Props {
  onSelect: (titleId: string, title: string) => void
  onClose: () => void
  existingIds: string[]
}

export default function BrowseOverlay({ onSelect, onClose, existingIds }: Props) {
  const [titles, setTitles] = useState<Title[]>([])

  useEffect(() => {
    fetch('/api/titles')
      .then((r) => (r.ok ? r.json() : []))
      .then(setTitles)
      .catch(() => {})
  }, [])

  return (
    <div className="absolute inset-0 z-50 bg-black/90 flex flex-col" onClick={onClose}>
      <div className="p-6" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-lg font-bold text-[#e5e2e1]">Add to tabs</h2>
          <button
            onClick={onClose}
            className="text-[#8e9285] hover:text-[#e5e2e1] text-2xl leading-none"
          >
            ×
          </button>
        </div>
        <div className="grid grid-cols-4 gap-3 overflow-y-auto max-h-[70vh]">
          {titles.map((t) => (
            <button
              key={t.id}
              onClick={() => onSelect(t.id, t.title)}
              className={`relative aspect-video rounded-lg overflow-hidden border-2 transition-all ${
                existingIds.includes(t.id)
                  ? 'border-[var(--color-accent)] opacity-60 cursor-default'
                  : 'border-transparent hover:border-[var(--color-accent)]/50 hover:scale-105'
              }`}
              disabled={existingIds.includes(t.id)}
            >
              <img
                src={`/api/titles/${t.id}/thumbnail/card`}
                alt=""
                className="w-full h-full object-cover"
                onError={(e) => {
                  ;(e.target as HTMLImageElement).style.display = 'none'
                }}
              />
              <div className="absolute inset-0 bg-gradient-to-t from-black/70 to-transparent" />
              <p className="absolute bottom-2 left-2 text-xs font-bold text-[#e5e2e1]">
                {t.title}
              </p>
              {existingIds.includes(t.id) && (
                <div className="absolute inset-0 flex items-center justify-center">
                  <span className="text-[var(--color-accent)] text-xs font-bold">Already open</span>
                </div>
              )}
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
