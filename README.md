# Collectarr

Collectarr is a self-hosted video library scanner and player. A Go backend scans a media directory, stores video metadata in SQLite, streams files with range support, generates thumbnails with ffmpeg, and serves the built Svelte frontend from the same container.

## Tech stack

- Go 1.22 backend with `gorilla/mux`
- SQLite for metadata storage
- ffmpeg/ffprobe for thumbnails and duration detection
- SvelteKit + Vite frontend
- Docker Compose for local deployment

## Quick start

1. Build the frontend assets:
   ```bash
   cd frontend
   npm install
   npm run build
   ```
2. From the project root, create an environment file if needed:
   ```bash
   cp .env.example .env
   ```
3. Start the app:
   ```bash
   docker compose up --build
   ```
4. Open `http://localhost:3000`.

The backend listens on port `8080` inside the container and is published as port `3000` on the host. API routes are available under `/api/*`, and the backend serves the frontend build from `/`.

## Environment variables

Docker Compose reads these values from `.env` or your shell:

- `MEDIA_HOST_PATH` — host path to your video library; mounted into the container at `/media`
- `MEDIA_PATH` — media path inside the container; defaults to `/media`
- `DB_PATH` — SQLite database path inside the container; defaults to `/data/collectarr.db`
- `PORT` — backend listen port inside the container; defaults to `8080`

Example `.env`:

```env
MEDIA_HOST_PATH=/Volumes/media/videos
MEDIA_PATH=/media
DB_PATH=/data/collectarr.db
PORT=8080
```

## Development setup

### Backend

Run the backend locally:

```bash
cd backend
go run .
```

Useful environment variables for local runs:

```bash
export MEDIA_PATH=/absolute/path/to/media
export DB_PATH=/absolute/path/to/collectarr.db
export PORT=8080
```

If you want duration probing and thumbnail generation outside Docker, install `ffmpeg` so both `ffmpeg` and `ffprobe` are on your `PATH`.

### Frontend

Run the frontend dev server separately:

```bash
cd frontend
npm install
npm run dev
```

Build production assets for Docker-backed serving:

```bash
cd frontend
npm run build
```

### Docker workflow

- Rebuild after backend or Dockerfile changes: `docker compose up --build`
- Rebuild frontend assets after UI changes: `cd frontend && npm run build`
- Persisted app data is stored in `./data`
