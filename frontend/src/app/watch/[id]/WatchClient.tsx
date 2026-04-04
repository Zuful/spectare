'use client'
import { useEffect, useRef, useState, useCallback } from 'react'
import Link from 'next/link'
import Hls from 'hls.js'
import { usePathname } from 'next/navigation'
import { usePlayerTabs } from '@/store/playerTabs'
import { streamUrl } from '@/lib/api'

function formatTime(sec: number): string {
  if (!isFinite(sec) || sec < 0) return '0:00'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  return `${m}:${String(s).padStart(2, '0')}`
}

type StreamMode = 'hls' | 'direct' | 'none'
type SubTrack = { lang: string; label: string; file: string }

export default function WatchClient({ id: staticId }: { id: string }) {
  // Use the actual URL path to get the real ID — the static export may serve a
  // template page (e.g. /watch/1/) for any unknown hex ID, so we must read the
  // real ID from the live URL rather than the baked-in prop.
  const pathname = usePathname() // e.g. "/watch/abc123def456"
  const id = pathname.split('/').filter(Boolean)[1] ?? staticId
  const videoRef = useRef<HTMLVideoElement>(null)
  const hlsRef = useRef<Hls | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const controlsTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const [playing, setPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [muted, setMuted] = useState(false)
  const [fullscreen, setFullscreen] = useState(false)
  const [streamMode, setStreamMode] = useState<StreamMode>('none')
  const [controlsVisible, setControlsVisible] = useState(true)
  const [titleName, setTitleName] = useState(`Title ${id}`)
  const [subtitles, setSubtitles] = useState<SubTrack[]>([])
  const [activeSub, setActiveSub] = useState<string | null>(null) // lang code or null
  const [showSubMenu, setShowSubMenu] = useState(false)
  const [castAvailable, setCastAvailable] = useState(false)
  const [casting, setCasting] = useState(false)

  const { tabs, activeTabId, openTab, closeTab, setActiveTab, updateCurrentTime } = usePlayerTabs()

  // Resolve stream mode and subtitles from API on mount
  useEffect(() => {
    fetch(`/api/titles/${id}`)
      .then((r) => r.ok ? r.json() : null)
      .then((data) => {
        if (!data) return
        if (data.title) setTitleName(data.title)
        openTab({ id, titleId: id, title: data.title ?? `Title ${id}`, thumbnail: '' })
        if (data.streamReady) {
          setStreamMode('hls')
        } else if (data.directPath) {
          setStreamMode('direct')
        } else {
          setStreamMode('none')
        }
      })
      .catch(() => {
        openTab({ id, titleId: id, title: `Title ${id}`, thumbnail: '' })
        setStreamMode('none')
      })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // Load subtitle tracks
  useEffect(() => {
    fetch(`/api/titles/${id}/subtitles`)
      .then((r) => r.ok ? r.json() : [])
      .then((tracks: SubTrack[]) => setSubtitles(tracks))
      .catch(() => {})
  }, [id])

  // Load Google Cast SDK (Chrome/Edge only — no-op in other browsers)
  useEffect(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const w = window as any
    w.__onGCastApiAvailable = (isAvailable: boolean) => {
      if (!isAvailable) return
      w.cast.framework.CastContext.getInstance().setOptions({
        receiverApplicationId: w.chrome.cast.media.DEFAULT_MEDIA_RECEIVER_APP_ID,
        autoJoinPolicy: w.chrome.cast.AutoJoinPolicy.ORIGIN_SCOPED,
      })
      w.cast.framework.CastContext.getInstance().addEventListener(
        w.cast.framework.CastContextEventType.SESSION_STATE_CHANGED,
        (e: { sessionState: string }) => {
          setCasting(e.sessionState === 'SESSION_STARTED' || e.sessionState === 'SESSION_RESUMED')
        }
      )
      setCastAvailable(true)
    }
    const script = document.createElement('script')
    script.src = 'https://www.gstatic.com/cv/js/sender/v1/cast_sender.js?loadCastFramework=1'
    document.head.appendChild(script)
    return () => { try { document.head.removeChild(script) } catch {} }
  }, [])

  const handleCast = useCallback(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const w = window as any
    const context = w.cast.framework.CastContext.getInstance()
    if (casting) {
      context.endCurrentSession(true)
      return
    }
    const baseUrl = window.location.origin
    const mediaUrl = streamMode === 'hls'
      ? `${baseUrl}/api/stream/${activeTitleId}/master.m3u8`
      : `${baseUrl}/api/stream/${activeTitleId}/direct`
    const contentType = streamMode === 'hls' ? 'application/x-mpegURL' : 'video/mp4'

    context.requestSession().then(() => {
      const session = context.getCurrentSession()
      if (!session) return
      const mediaInfo = new w.chrome.cast.media.MediaInfo(mediaUrl, contentType)
      const meta = new w.chrome.cast.media.MovieMediaMetadata()
      meta.title = titleName
      meta.images = [{ url: `${baseUrl}/api/titles/${activeTitleId}/thumbnail/backdrop` }]
      mediaInfo.metadata = meta
      session.loadMedia(new w.chrome.cast.media.LoadRequest(mediaInfo))
    }).catch(() => {})
  }, [casting, streamMode, activeTitleId, titleName])

  // Sync active subtitle track on the <video> element
  useEffect(() => {
    const video = videoRef.current
    if (!video) return
    for (let i = 0; i < video.textTracks.length; i++) {
      const t = video.textTracks[i]
      t.mode = activeSub && t.language === activeSub ? 'showing' : 'hidden'
    }
  }, [activeSub])

  const activeTab = tabs.find((t) => t.id === activeTabId)
  const activeTitleId = activeTab?.titleId ?? id

  // Load stream whenever active title or stream mode changes
  useEffect(() => {
    const video = videoRef.current
    if (!video || streamMode === 'none') return

    if (hlsRef.current) {
      hlsRef.current.destroy()
      hlsRef.current = null
    }

    setPlaying(false)
    setCurrentTime(0)
    setDuration(0)

    const savedTime = tabs.find((t) => t.titleId === activeTitleId)?.currentTime ?? 0

    if (streamMode === 'hls') {
      const url = streamUrl(activeTitleId)
      if (Hls.isSupported()) {
        const hls = new Hls({ startLevel: -1 })
        hlsRef.current = hls
        hls.loadSource(url)
        hls.attachMedia(video)
        hls.on(Hls.Events.MANIFEST_PARSED, () => {
          if (savedTime > 0) video.currentTime = savedTime
        })
        hls.on(Hls.Events.ERROR, (_, data) => {
          if (data.fatal) setStreamMode('none')
        })
      } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
        video.src = url
        if (savedTime > 0) video.addEventListener('loadedmetadata', () => { video.currentTime = savedTime }, { once: true })
      } else {
        setStreamMode('none')
      }
    } else if (streamMode === 'direct') {
      video.src = `/api/stream/${activeTitleId}/direct`
      if (savedTime > 0) video.addEventListener('loadedmetadata', () => { video.currentTime = savedTime }, { once: true })
    }

    return () => {
      if (hlsRef.current) { hlsRef.current.destroy(); hlsRef.current = null }
    }
  }, [activeTitleId, streamMode]) // eslint-disable-line react-hooks/exhaustive-deps

  // Controls auto-hide
  const showControls = useCallback(() => {
    setControlsVisible(true)
    if (controlsTimerRef.current) clearTimeout(controlsTimerRef.current)
    controlsTimerRef.current = setTimeout(() => {
      if (videoRef.current && !videoRef.current.paused) setControlsVisible(false)
    }, 3000)
  }, [])

  useEffect(() => () => { if (controlsTimerRef.current) clearTimeout(controlsTimerRef.current) }, [])

  // Keyboard shortcuts
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.target as HTMLElement).tagName === 'INPUT') return
      const video = videoRef.current
      if (!video) return
      switch (e.key) {
        case ' ': case 'k': e.preventDefault(); video.paused ? video.play().catch(() => {}) : video.pause(); break
        case 'f': case 'F': e.preventDefault(); handleToggleFullscreen(); break
        case 'm': case 'M': e.preventDefault(); video.muted = !video.muted; setMuted(video.muted); break
        case 'ArrowLeft': e.preventDefault(); video.currentTime = Math.max(0, video.currentTime - 10); break
        case 'ArrowRight': e.preventDefault(); video.currentTime = Math.min(video.duration || 0, video.currentTime + 10); break
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    const handler = () => setFullscreen(!!document.fullscreenElement)
    document.addEventListener('fullscreenchange', handler)
    return () => document.removeEventListener('fullscreenchange', handler)
  }, [])

  const handleTogglePlay = useCallback(() => {
    const v = videoRef.current
    if (!v) return
    v.paused ? v.play().catch(() => {}) : v.pause()
  }, [])

  const handleToggleMute = useCallback(() => {
    const v = videoRef.current
    if (!v) return
    v.muted = !v.muted
    setMuted(v.muted)
  }, [])

  const handleToggleFullscreen = useCallback(() => {
    const el = containerRef.current
    if (!el) return
    if (!document.fullscreenElement) el.requestFullscreen().catch(() => {})
    else document.exitFullscreen().catch(() => {})
  }, [])

  const handleSeek = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    const v = videoRef.current
    if (!v || !duration) return
    const rect = e.currentTarget.getBoundingClientRect()
    v.currentTime = ((e.clientX - rect.left) / rect.width) * duration
  }, [duration])

  const handleSwitchTab = useCallback((tabId: string) => {
    if (activeTabId && videoRef.current) updateCurrentTime(activeTabId, videoRef.current.currentTime)
    setActiveTab(tabId)
  }, [activeTabId, setActiveTab, updateCurrentTime])

  const handleCloseTab = useCallback((e: React.MouseEvent, tabId: string) => {
    e.stopPropagation()
    closeTab(tabId)
  }, [closeTab])

  const handleContainerClick = useCallback(() => {
    if (showSubMenu) { setShowSubMenu(false); return }
    handleTogglePlay()
  }, [showSubMenu, handleTogglePlay])

  const progress = duration > 0 ? (currentTime / duration) * 100 : 0
  const overlayHidden = playing && !controlsVisible

  return (
    <div
      ref={containerRef}
      className="h-screen bg-black flex flex-col overflow-hidden select-none"
      onMouseMove={showControls}
      style={{ cursor: overlayHidden ? 'none' : 'default' }}
    >
      {/* Breadcrumb */}
      <div className={`flex items-center gap-3 px-6 py-3 z-10 bg-gradient-to-b from-black/60 to-transparent transition-opacity duration-300 ${overlayHidden ? 'opacity-0' : 'opacity-100'}`}>
        <Link href={`/title/${activeTitleId}`} className="flex items-center gap-2 text-[#8e9285] hover:text-[#e5e2e1] transition-colors text-sm">
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="m15 18-6-6 6-6"/>
          </svg>
          Back to {titleName}
        </Link>
        {streamMode === 'direct' && (
          <span className="text-[10px] font-mono text-[#454545] ml-2">DIRECT</span>
        )}
        {streamMode === 'hls' && (
          <span className="text-[10px] font-mono text-[#87a96b] ml-2">HLS</span>
        )}
      </div>

      {/* Video area */}
      <div className="flex-1 bg-[#0a0a0a] relative overflow-hidden" onClick={handleContainerClick}>
        <video
          ref={videoRef}
          className="w-full h-full object-contain"
          onTimeUpdate={() => {
            const v = videoRef.current; if (!v) return
            setCurrentTime(v.currentTime)
            if (activeTabId) updateCurrentTime(activeTabId, v.currentTime)
          }}
          onDurationChange={() => setDuration(videoRef.current?.duration ?? 0)}
          onPlay={() => { setPlaying(true); showControls() }}
          onPause={() => { setPlaying(false); setControlsVisible(true) }}
          playsInline
          crossOrigin="anonymous"
        >
          {subtitles.map((s) => (
            <track
              key={s.lang}
              kind="subtitles"
              src={`/api/titles/${id}/subtitles/${s.lang}.vtt`}
              srcLang={s.lang}
              label={s.label}
              default={s.lang === activeSub}
            />
          ))}
        </video>

        {/* Unavailable overlay */}
        {streamMode === 'none' && (
          <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
            <div className="text-center text-[#353534]">
              <div className="text-6xl mb-4">▶</div>
              <p className="text-sm font-medium text-[#454545]">No stream available</p>
              <p className="text-xs mt-1">Go to the title page to add a source or transcode</p>
            </div>
          </div>
        )}

        {/* Controls overlay */}
        <div className={`absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/80 via-black/20 to-transparent px-6 py-4 transition-opacity duration-300 ${overlayHidden ? 'opacity-0 pointer-events-none' : 'opacity-100'}`}>
          {/* Seek bar */}
          <div
            className="w-full h-1 bg-[#ffffff20] rounded-full mb-3 cursor-pointer group"
            onClick={(e) => { e.stopPropagation(); handleSeek(e) }}
          >
            <div className="h-full bg-[#87a96b] rounded-full" style={{ width: `${progress}%` }} />
          </div>

          <div className="flex items-center gap-4 text-[#e5e2e1]">
            <button className="hover:text-[#87a96b] transition-colors text-lg leading-none w-6 flex items-center justify-center"
              onClick={(e) => { e.stopPropagation(); handleTogglePlay() }}>
              {playing
                ? <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="4" width="4" height="16"/><rect x="14" y="4" width="4" height="16"/></svg>
                : <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><polygon points="5,3 19,12 5,21"/></svg>
              }
            </button>
            <span className="text-xs text-[#8e9285] font-mono tabular-nums">{formatTime(currentTime)} / {formatTime(duration)}</span>
            <div className="flex-1" />
            <button className="text-[#8e9285] hover:text-[#e5e2e1] transition-colors"
              onClick={(e) => { e.stopPropagation(); handleToggleMute() }}>
              {muted
                ? <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polygon points="11,5 6,9 2,9 2,15 6,15 11,19"/><line x1="23" y1="9" x2="17" y2="15"/><line x1="17" y1="9" x2="23" y2="15"/></svg>
                : <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polygon points="11,5 6,9 2,9 2,15 6,15 11,19"/><path d="M15.54,8.46a5,5,0,0,1,0,7.07"/><path d="M19.07,4.93a10,10,0,0,1,0,14.14"/></svg>
              }
            </button>
            {/* CC / subtitle selector */}
            <div className="relative">
              <button
                className={`text-xs font-mono transition-colors px-1 rounded ${activeSub ? 'text-[#87a96b] bg-[#87a96b]/10' : subtitles.length > 0 ? 'text-[#8e9285] hover:text-[#e5e2e1]' : 'text-[#353534] cursor-default'}`}
                onClick={(e) => { e.stopPropagation(); if (subtitles.length > 0) setShowSubMenu((v) => !v) }}
                title={subtitles.length === 0 ? 'No subtitles available' : 'Subtitles'}
              >
                CC
              </button>
              {showSubMenu && (
                <div className="absolute bottom-8 right-0 bg-[#1c1b1b] border border-[#2a2a2a] rounded-lg overflow-hidden shadow-xl min-w-[130px] z-50">
                  <button
                    className={`w-full text-left px-3 py-2 text-xs transition-colors ${!activeSub ? 'text-[#87a96b]' : 'text-[#8e9285] hover:text-[#e5e2e1] hover:bg-[#2a2a2a]'}`}
                    onClick={(e) => { e.stopPropagation(); setActiveSub(null); setShowSubMenu(false) }}
                  >
                    Off
                  </button>
                  {subtitles.map((s) => (
                    <button
                      key={s.lang}
                      className={`w-full text-left px-3 py-2 text-xs transition-colors ${activeSub === s.lang ? 'text-[#87a96b]' : 'text-[#8e9285] hover:text-[#e5e2e1] hover:bg-[#2a2a2a]'}`}
                      onClick={(e) => { e.stopPropagation(); setActiveSub(s.lang); setShowSubMenu(false) }}
                    >
                      {s.label}
                    </button>
                  ))}
                </div>
              )}
            </div>
            {castAvailable && streamMode !== 'none' && (
              <button
                title={casting ? 'Stop casting' : 'Cast to TV'}
                className={`transition-colors ${casting ? 'text-[#87a96b]' : 'text-[#8e9285] hover:text-[#e5e2e1]'}`}
                onClick={(e) => { e.stopPropagation(); handleCast() }}
              >
                {/* Google Cast icon */}
                <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M1 18v3h3c0-1.66-1.34-3-3-3zm0-4v2c2.76 0 5 2.24 5 5h2c0-3.87-3.13-7-7-7zm0-4v2c4.97 0 9 4.03 9 9h2C12 14.36 7.03 9 1 10zm20-7H3C1.9 3 1 3.9 1 5v3h2V5h18v14h-7v2h7c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2z"/>
                </svg>
              </button>
            )}
            <button className="text-[#8e9285] hover:text-[#e5e2e1] transition-colors"
              onClick={(e) => { e.stopPropagation(); handleToggleFullscreen() }}>
              {fullscreen
                ? <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M8 3v3a2 2 0 0 1-2 2H3"/><path d="M21 8h-3a2 2 0 0 1-2-2V3"/><path d="M3 16h3a2 2 0 0 1 2 2v3"/><path d="M16 21v-3a2 2 0 0 1 2-2h3"/></svg>
                : <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M8 3H5a2 2 0 0 0-2 2v3"/><path d="M21 8V5a2 2 0 0 0-2-2h-3"/><path d="M3 16v3a2 2 0 0 0 2 2h3"/><path d="M16 21h3a2 2 0 0 0 2-2v-3"/></svg>
              }
            </button>
          </div>
        </div>
      </div>

      {/* Keyboard hints */}
      <div className="px-6 py-1 flex gap-6 text-[10px] text-[#353534] font-mono bg-[#0a0a0a] shrink-0">
        {[['SPACE', 'Play/Pause'], ['F', 'Fullscreen'], ['M', 'Mute'], ['←→', 'Seek 10s']].map(([key, desc]) => (
          <span key={key}><span className="text-[#454652]">{key}</span> {desc}</span>
        ))}
      </div>

      {/* Multi-tab bar */}
      <div className="bg-[#0e0e0e] border-t border-[#1c1b1b] flex items-center gap-1 px-2 h-14 overflow-x-auto shrink-0">
        {tabs.map((tab) => {
          const isActive = tab.id === activeTabId
          return (
            <button key={tab.id} onClick={() => handleSwitchTab(tab.id)}
              className={`flex items-center gap-2 px-3 py-1.5 rounded text-sm min-w-0 shrink-0 transition-colors border-b-2 ${isActive ? 'bg-[#1c1b1b] border-[#87a96b]' : 'hover:bg-[#161616] border-transparent'}`}>
              <div className="w-10 aspect-video bg-[#2a2a2a] rounded shrink-0 overflow-hidden">
                {tab.thumbnail && <img src={tab.thumbnail} alt="" className="w-full h-full object-cover" />}
              </div>
              <span className="text-[#e5e2e1] font-medium truncate text-xs max-w-[100px]">{tab.title}</span>
              <span role="button" className="text-[#8e9285] hover:text-[#e5e2e1] ml-1 shrink-0 text-sm leading-none"
                onClick={(e) => handleCloseTab(e, tab.id)}>×</span>
            </button>
          )
        })}
        <Link href="/browse"
          className="flex items-center justify-center w-8 h-8 text-[#8e9285] hover:text-[#e5e2e1] hover:bg-[#1c1b1b] rounded transition-colors ml-1 text-lg shrink-0"
          title="Add title">+</Link>
      </div>
    </div>
  )
}
