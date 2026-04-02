'use client'
import { useRef } from 'react'

type Props = {
  label: string
  hint: string
  aspectClass: string        // e.g. 'aspect-video' | 'aspect-[2/3]' | 'aspect-[21/9]'
  preview: string | null
  onChange: (file: File, preview: string) => void
  onClear: () => void
  disabled?: boolean
}

export default function ThumbnailZone({ label, hint, aspectClass, preview, onChange, onClear, disabled }: Props) {
  const ref = useRef<HTMLInputElement>(null)

  return (
    <div>
      <p className="text-[10px] text-[#8e9285] uppercase tracking-widest mb-1.5">{label}</p>
      <div
        className={`relative ${aspectClass} rounded-lg overflow-hidden border-2 border-dashed cursor-pointer transition-colors group ${
          preview ? 'border-[#87a96b]/40' : 'border-[#2a2a2a] hover:border-[#43483d]'
        } ${disabled ? 'opacity-50 pointer-events-none' : ''}`}
        onClick={() => ref.current?.click()}
      >
        {preview ? (
          <>
            <img src={preview} alt="" className="w-full h-full object-cover" />
            <div className="absolute inset-0 bg-black/0 group-hover:bg-black/40 transition-colors flex items-center justify-center">
              <span className="opacity-0 group-hover:opacity-100 text-xs text-white font-medium transition-opacity">Change</span>
            </div>
          </>
        ) : (
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-1 text-[#353534]">
            <span className="text-2xl">+</span>
          </div>
        )}
        <input
          ref={ref}
          type="file"
          accept="image/*"
          className="hidden"
          disabled={disabled}
          onChange={(e) => {
            const f = e.target.files?.[0]
            if (!f) return
            onChange(f, URL.createObjectURL(f))
            e.target.value = ''
          }}
        />
      </div>
      <p className="text-[9px] text-[#454545] mt-1">{hint}</p>
      {preview && !disabled && (
        <button type="button" onClick={onClear} className="text-[9px] text-[#454545] hover:text-red-400 transition-colors mt-0.5">
          × Supprimer
        </button>
      )}
    </div>
  )
}
