# Spectare

A premium VOD streaming platform — part of a video suite alongside [sub-one](https://github.com/Zuful/sub-one) and [captura](https://github.com/Zuful/captura).

Dark, cinematic UI inspired by Netflix/Disney+/Prime Video, built around a matcha green design system ("Spectare Cinematic"). Signature feature: multi-tab video player — watch multiple titles simultaneously and switch between them like browser tabs.

## Requirements

- **Go** 1.19+
- **Node.js** 18+ and **npm**
- **ffmpeg** (for HLS transcoding — coming soon)

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
# or with a custom port:
PORT=9000 ./spectare
```

Opens at `http://localhost:8766`.

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
| Player | HLS.js (streaming) |
| Multi-tab state | Zustand |
| Backend | Go + chi |
| Transcoding | ffmpeg → HLS segments *(coming soon)* |
| Database | PostgreSQL *(coming soon)* |
| Storage | Local filesystem → MinIO/S3 later |
| Mobile | React Native / Expo *(coming soon)* |

## Routes

| Page | Path |
|------|------|
| Home | `/` |
| Browse / Catalogue | `/browse` |
| My List | `/my-list` |
| Title detail | `/title/[id]` |
| Player | `/watch/[id]` |

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/titles` | List all titles |
| `GET` | `/api/titles/{id}` | Get title metadata |
| `GET` | `/api/stream/{id}/master.m3u8` | HLS stream *(not yet implemented)* |

## Design system

**Spectare Cinematic** — Stitch project `9255986249033920344`

- Primary: matcha green `#87A96B`
- Background: obsidian `#131313`
- Typography: Manrope (display) + Inter (UI)
- Philosophy: "Sophisticated Silence" — UI recedes so content leads
