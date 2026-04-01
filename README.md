# Spectare

A premium VOD streaming platform — part of a video suite alongside [sub-one](https://github.com/Zuful/sub-one) and [captura](https://github.com/Zuful/captura).

Dark, cinematic UI inspired by Netflix/Disney+/Prime Video, built around a matcha green design system ("Spectare Cinematic"). Signature feature: multi-tab video player — watch multiple titles simultaneously and switch between them like browser tabs.

## Screenshots

| Home | Browse |
|------|--------|
| ![Home](docs/screen-home.png) | ![Browse](docs/screen-browse.png) |

| Player (multi-tab) | Upload |
|--------------------|--------|
| ![Player](docs/screen-player.png) | ![Upload](docs/screen-upload.png) |

## Requirements

- **Go** 1.19+
- **Node.js** 18+ and **npm**
- **ffmpeg** + **ffprobe** (for HLS transcoding)

## Build

```bash
make build
```

Compiles the Next.js frontend to a static export and embeds it into a single Go binary.

Without make:

```bash
cd frontend && npm install && npm run build && cd ..
go build -o spectare .
```

## Run

```bash
./spectare
# or with a custom port, data directory, and media source:
PORT=9000 DATA_DIR=/var/spectare MEDIA_DIR=/Volumes/MyDrive/Movies ./spectare
```

Opens at `http://localhost:8766`.

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8766` | HTTP listen port |
| `DATA_DIR` | `./data` | Where title metadata and HLS segments are stored |
| `MEDIA_DIR` | *(unset)* | Folder to scan for videos on startup (supports removable media) |

If `MEDIA_DIR` is set, Spectare scans it at boot and registers all video files — no upload needed. You can rescan at any time via `POST /api/scan`. Videos on external drives are served directly; if the drive is disconnected, the stream returns 404 until reconnected.

## Development

```bash
# Terminal 1 — Go API server
make dev-backend

# Terminal 2 — Next.js with hot reload
make dev-frontend
```

The Next.js dev server proxies `/api` to `localhost:8766`.

## Stack

| Layer | Tech |
|-------|------|
| Frontend | Next.js 16 + Tailwind CSS v4 |
| Player | HLS.js (adaptive streaming) |
| Multi-tab state | Zustand |
| Backend | Go + chi |
| Transcoding | ffmpeg → HLS segments (360p + 720p) |
| Database | PostgreSQL *(coming soon)* |
| Storage | Local filesystem → MinIO/S3 later |
| Mobile | React Native / Expo *(coming soon)* |

## Workflow

### Option A — Upload via browser

1. Go to `/admin/upload`
2. Drop a video file (MP4, MKV, AVI, MOV, WebM — up to 8 GB)
3. Fill in metadata (title, year, genre, type…)
4. Click **Upload** — the file is immediately watchable via direct streaming
5. Optionally enable **Transcode to HLS** — ffmpeg generates 360p + 720p adaptive renditions in the background
6. When transcoding completes, the player switches to adaptive HLS automatically

### Option B — Point at a folder

```bash
MEDIA_DIR=/path/to/videos ./spectare
```

All video files are scanned on startup. Titles + years are parsed from filenames (e.g. `Blade.Runner.2049.mkv` → *Blade Runner*, 2049). Videos are playable immediately; you can trigger HLS transcoding per-title from the title page.

## Data layout

```
data/
  titles/
    {id}/
      meta.json          ← title metadata + directPath (absolute path to source)
      original/
        video.*          ← source file (uploaded titles only)
      hls/               ← only present after HLS transcoding
        master.m3u8      ← adaptive playlist
        360p/
          stream.m3u8
          seg000.ts  …
        720p/
          stream.m3u8
          seg000.ts  …
```

Titles sourced via `MEDIA_DIR` store only `meta.json` — the source file stays in place on the original drive.

## Routes

| Page | Path |
|------|------|
| Home | `/` |
| Browse / Catalogue | `/browse` |
| My List | `/my-list` |
| Title detail | `/title/[id]` |
| Player | `/watch/[id]` |
| Upload | `/admin/upload` |

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/titles` | List all titles |
| `POST` | `/api/titles` | Upload video + metadata (multipart/form-data) |
| `GET` | `/api/titles/{id}` | Get title metadata |
| `GET` | `/api/titles/{id}/status` | Transcoding progress `{status, progress}` |
| `POST` | `/api/titles/{id}/transcode` | Trigger HLS transcoding for an existing title |
| `GET` | `/api/stream/{id}/direct` | Stream source file directly (Range requests supported) |
| `GET` | `/api/stream/{id}/master.m3u8` | HLS master playlist |
| `GET` | `/api/stream/{id}/{quality}/stream.m3u8` | Variant playlist |
| `GET` | `/api/stream/{id}/{quality}/{segment}.ts` | Video segment |
| `POST` | `/api/scan` | Rescan `MEDIA_DIR` (or pass `dir=` in body to override) |

## Design system

**Spectare Cinematic** — Stitch project `9255986249033920344`

- Primary: matcha green `#87A96B`
- Background: obsidian `#131313`
- Typography: Manrope (display) + Inter (UI)
- Philosophy: "Sophisticated Silence" — UI recedes so content leads
