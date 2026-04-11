const BASE = process.env.NEXT_PUBLIC_API_URL ?? '/api'

export interface Title {
  id: string
  title: string
  year: number
  genre: string[]
  type: 'movie' | 'series'
  rating: string
  poster: string
}

export interface TitleDetail extends Title {
  synopsis: string
  director: string
  cast: string[]
  seasons?: number
  streamReady: boolean
}

export async function fetchTitles(): Promise<Title[]> {
  const res = await fetch(`${BASE}/titles`)
  if (!res.ok) throw new Error('Failed to fetch titles')
  return res.json()
}

export async function fetchTitle(id: string): Promise<TitleDetail> {
  const res = await fetch(`${BASE}/titles/${id}`)
  if (!res.ok) throw new Error('Failed to fetch title')
  return res.json()
}

export function streamUrl(id: string): string {
  return `${BASE}/stream/${id}/master.m3u8`
}
