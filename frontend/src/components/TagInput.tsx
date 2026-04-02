'use client'
import { useState, useRef, KeyboardEvent } from 'react'

const SUGGESTIONS = [
  'Action', 'Adventure', 'Animation', 'Comedy', 'Crime', 'Documentary',
  'Drama', 'Fantasy', 'Horror', 'Musical', 'Mystery', 'Romance',
  'Sci-Fi', 'Thriller', 'Western',
]

type Props = {
  tags: string[]
  onChange: (tags: string[]) => void
  disabled?: boolean
}

export default function TagInput({ tags, onChange, disabled }: Props) {
  const [input, setInput] = useState('')
  const [focused, setFocused] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  const addTag = (raw: string) => {
    const value = raw.trim()
    if (!value || tags.includes(value)) return
    onChange([...tags, value])
    setInput('')
  }

  const handleKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault()
      addTag(input)
    } else if (e.key === 'Backspace' && input === '' && tags.length > 0) {
      onChange(tags.slice(0, -1))
    }
  }

  const suggestions = SUGGESTIONS.filter(
    (s) => !tags.includes(s) && s.toLowerCase().startsWith(input.toLowerCase())
  )

  return (
    <div className="relative">
      <div
        className={`flex flex-wrap gap-1.5 min-h-[42px] bg-[#1c1b1b] border rounded-lg px-3 py-2 cursor-text transition-colors ${
          focused ? 'border-[#87a96b]/50' : 'border-[#2a2a2a]'
        } ${disabled ? 'opacity-50 pointer-events-none' : ''}`}
        onClick={() => inputRef.current?.focus()}
      >
        {tags.map((tag) => (
          <span
            key={tag}
            className="flex items-center gap-1 bg-[#87a96b]/15 text-[#87a96b] text-xs font-medium px-2 py-0.5 rounded-full"
          >
            {tag}
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); onChange(tags.filter((t) => t !== tag)) }}
              className="hover:text-[#e5e2e1] leading-none text-sm"
            >
              ×
            </button>
          </span>
        ))}
        <input
          ref={inputRef}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKey}
          onFocus={() => setFocused(true)}
          onBlur={() => { setFocused(false); if (input.trim()) addTag(input) }}
          placeholder={tags.length === 0 ? 'Drama, Sci-Fi…' : ''}
          className="bg-transparent outline-none text-sm text-[#e5e2e1] placeholder:text-[#8e9285] min-w-[80px] flex-1"
        />
      </div>

      {/* Suggestions dropdown */}
      {focused && input.length > 0 && suggestions.length > 0 && (
        <div className="absolute z-20 top-full mt-1 left-0 right-0 bg-[#1c1b1b] border border-[#2a2a2a] rounded-lg overflow-hidden shadow-lg">
          {suggestions.slice(0, 6).map((s) => (
            <button
              key={s}
              type="button"
              onMouseDown={(e) => { e.preventDefault(); addTag(s) }}
              className="w-full text-left px-3 py-2 text-sm text-[#c4c8ba] hover:bg-[#2a2a2a] hover:text-[#e5e2e1] transition-colors"
            >
              {s}
            </button>
          ))}
        </div>
      )}

      {/* Quick suggestions when empty and focused */}
      {focused && input.length === 0 && tags.length === 0 && (
        <div className="flex flex-wrap gap-1.5 mt-2">
          {SUGGESTIONS.slice(0, 8).map((s) => (
            <button
              key={s}
              type="button"
              onMouseDown={(e) => { e.preventDefault(); addTag(s) }}
              className="text-[10px] px-2 py-0.5 rounded-full bg-[#2a2a2a] text-[#8e9285] hover:bg-[#87a96b]/15 hover:text-[#87a96b] transition-colors"
            >
              {s}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
