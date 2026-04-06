'use client'
import React, { useEffect, useRef } from 'react'
import Hls from 'hls.js'
import { saveProgress, getProgress, markWatched, setLastPlayed } from '@/lib/watchProgress'

type SubTrack = { lang: string; label: string; file: string }

interface VideoPlayerProps {
  titleId: string
  isActive: boolean
  activeSub: string | null
  subtitles: SubTrack[]
  onTimeUpdate: (time: number, duration: number) => void
  onDurationChange: (duration: number) => void
  onPlay: () => void
  onPause: () => void
  onEnded: () => void
  onStreamMode: (mode: 'hls' | 'direct' | 'none') => void
  videoRef: React.MutableRefObject<HTMLVideoElement | null>
}

export default function VideoPlayer({
  titleId,
  isActive,
  activeSub,
  subtitles,
  onTimeUpdate,
  onDurationChange,
  onPlay,
  onPause,
  onEnded,
  onStreamMode,
  videoRef,
}: VideoPlayerProps) {
  const hlsRef = useRef<Hls | null>(null)
  const lastSaveRef = useRef(0)

  // Load stream on mount
  useEffect(() => {
    fetch(`/api/titles/${titleId}`)
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (!data) {
          onStreamMode('none')
          return
        }
        const mode: 'hls' | 'direct' | 'none' = data.streamReady
          ? 'hls'
          : data.directPath
          ? 'direct'
          : 'none'
        onStreamMode(mode)

        const video = videoRef.current
        if (!video) return

        const saved = getProgress(titleId)
        const savedTime = saved?.currentTime ?? 0

        if (hlsRef.current) {
          hlsRef.current.destroy()
          hlsRef.current = null
        }

        if (mode === 'hls') {
          const url = `/api/stream/${titleId}/master.m3u8`
          if (Hls.isSupported()) {
            const hls = new Hls({ startLevel: -1 })
            hlsRef.current = hls
            hls.loadSource(url)
            hls.attachMedia(video)
            hls.on(Hls.Events.MANIFEST_PARSED, () => {
              if (savedTime > 0) video.currentTime = savedTime
            })
            hls.on(Hls.Events.ERROR, (_, data) => {
              if (data.fatal) onStreamMode('none')
            })
          } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
            video.src = url
            if (savedTime > 0)
              video.addEventListener(
                'loadedmetadata',
                () => {
                  video.currentTime = savedTime
                },
                { once: true }
              )
          }
        } else if (mode === 'direct') {
          video.src = `/api/stream/${titleId}/direct`
          if (savedTime > 0)
            video.addEventListener(
              'loadedmetadata',
              () => {
                video.currentTime = savedTime
              },
              { once: true }
            )
        }
      })
      .catch(() => onStreamMode('none'))

    return () => {
      if (hlsRef.current) {
        hlsRef.current.destroy()
        hlsRef.current = null
      }
    }
  }, [titleId]) // eslint-disable-line react-hooks/exhaustive-deps

  // Pause when tab becomes inactive
  useEffect(() => {
    if (!isActive) {
      videoRef.current?.pause()
    }
  }, [isActive]) // eslint-disable-line react-hooks/exhaustive-deps

  // Sync subtitle track
  useEffect(() => {
    const video = videoRef.current
    if (!video) return
    for (let i = 0; i < video.textTracks.length; i++) {
      const t = video.textTracks[i]
      t.mode = activeSub && t.language === activeSub ? 'showing' : 'hidden'
    }
  }, [activeSub]) // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <video
      ref={videoRef}
      className="w-full h-full object-contain"
      style={{ display: isActive ? 'block' : 'none' }}
      onTimeUpdate={() => {
        const v = videoRef.current
        if (!v) return
        onTimeUpdate(v.currentTime, v.duration)
        // Save progress every 5s
        const now = Date.now()
        if (now - lastSaveRef.current > 5000) {
          lastSaveRef.current = now
          if (v.duration > 0) saveProgress(titleId, v.currentTime, v.duration)
          if (v.currentTime / v.duration >= 0.9) markWatched(titleId)
        }
      }}
      onDurationChange={() => onDurationChange(videoRef.current?.duration ?? 0)}
      onPlay={() => { setLastPlayed(titleId); onPlay() }}
      onPause={onPause}
      onEnded={onEnded}
      playsInline
      crossOrigin="anonymous"
    >
      {subtitles.map((s) => (
        <track
          key={s.lang}
          kind="subtitles"
          src={`/api/titles/${titleId}/subtitles/${s.lang}.vtt`}
          srcLang={s.lang}
          label={s.label}
          default={s.lang === activeSub}
        />
      ))}
    </video>
  )
}
