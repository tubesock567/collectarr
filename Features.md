# Collectarr Features

A self-hosted video library scanner and player with a Go backend and Svelte frontend.

## Core Features

### Video Library Management

- **Media Scanning**: Automatically scans directories for video files (.mp4, .mkv, .avi, .mov, .webm, .m4v)
- **Metadata Extraction**: Uses ffprobe to extract video duration and resolution
- **Quality Detection**: Automatically detects video quality from filenames (720p, 1080p, 4K) or probes actual resolution
- **Video Grouping**: Groups multiple quality versions of the same title together
- **Database Storage**: SQLite backend for fast metadata storage and retrieval

### Video Playback

- **HTTP Range Support**: Stream videos with byte-range requests for seeking
- **Custom Video Player**: Built-in player with timeline scrubbing
- **Thumbnail Generation**: Auto-generated thumbnails using ffmpeg at intelligent timestamps
- **Preview Sprites**: Scrubber preview images showing video frames at intervals
- **Hover Previews**: Video preview clips on thumbnail hover

### Playlist Management

- **Create Playlists**: Create custom playlists with names and descriptions
- **Add/Remove Videos**: Dynamically manage playlist contents
- **Reorder Items**: Change video order within playlists
- **Playlist Covers**: Auto-generated cover images from playlist videos
- **Playlist Viewing**: Dedicated playlist browsing and playback

### Metadata Management

- **Tags**: Organize videos with custom tags
- **Actors**: Track performers/actors in videos
- **Bulk Editing**: Update metadata for multiple videos simultaneously
- **Metadata Catalog**: Global management of available tags and actors
- **Smart Completion**: Token-based input with autocomplete

### User Authentication

- **Session-based Auth**: Secure cookie-based authentication
- **Password Management**: Change passwords with bcrypt hashing
- **Protected Routes**: All API endpoints require authentication

### Settings & Configuration

- **Media Path**: Configure and browse media directories
- **Generation Settings**: Control thumbnail/preview auto-generation
  - Toggle thumbnail generation
  - Toggle scrubber sprite generation
  - Toggle hover preview generation
  - Auto-generate during scans
- **Database Management**: Clear database and reset library
- **Log Viewer**: View application logs (last 200 entries)

### Torrent Integration

- **Torrent Search**: Search across configured torrent indexers
- **Indexer Management**: Add/remove torrent site configurations
- **Download History**: Track downloaded torrents
- **History Management**: Clear download history

### Administration

- **System Logs**: Structured logging with log buffer
- **Database Reset**: Clear all video metadata
- **Manual Scanning**: Trigger library scans on-demand
- **Thumbnail Generation**: Bulk thumbnail generation

## Technical Features

### Backend (Go)

- **RESTful API**: Clean API design with gorilla/mux
- **CORS Support**: Cross-origin request handling
- **Request Logging**: Comprehensive HTTP request logging
- **SPA Serving**: Serves built Svelte frontend as single-page application
- **Concurrent Processing**: Background thumbnail/preview generation
- **File Type Detection**: MIME type detection for proper streaming
- **Path Security**: Prevents directory traversal attacks

### Frontend (SvelteKit)

- **Single Page Application**: Fast navigation without page reloads
- **Responsive Design**: Tailwind CSS with mobile support
- **Theme Support**: Dark/light mode with persistent preferences
- **Component Architecture**: Reusable Svelte components
  - VideoCard: Video display with hover effects
  - MetadataTokenInput: Tag/actor input with autocomplete
  - DirectoryBrowser: File system navigation
- **State Management**: Auth and preferences stores

### DevOps

- **Docker Support**: Full containerization with Docker Compose
- **Environment Configuration**: Flexible env-based configuration
- **Development Mode**: Hot-reload frontend dev server
- **Production Builds**: Optimized static builds
