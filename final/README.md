# Video Streaming Platform

A live video streaming platform similar to Twitch, designed to allow content creators to broadcast live video streams to viewers with low latency and high quality.

## System Overview

This platform enables streamers to share their content in real-time while viewers can watch, interact, and engage with both the streamer and other community members.

The system consists of two main components:
- **Backend**: Written in Go, handling video ingestion, stream management, and API services
- **Frontend**: Built with Next.js, providing the user interface for viewers and streamers

## Key Features Implemented

1. **User Authentication** - Register and login functionality
2. **Stream Management** - Create, list, and view streams
3. **Follow System** - Follow/unfollow channels and receive notifications
4. **Video Delivery** - HLS video delivery system using S3 for storage

## Architecture

The system follows a microservice architecture with the following components:

- **Video Service**: Handles stream metadata and status
- **Transcode Service**: Processes incoming RTMP streams to HLS format
- **Storage Service**: Manages video segments in cloud storage
- **User Service**: Handles user authentication and profile management
- **Follow Service**: Manages channel subscriptions and notifications

## Getting Started

### Prerequisites

- Go 1.20 or higher
- Node.js 18 or higher
- MongoDB
- AWS S3 account (or local alternative)
- FFMPEG installed on the system

### Running the Backend

```bash
cd backend
go run ./cmd/server/main.go
```

The server will start on http://localhost:8080
docker run --rm -it \
-e MTX_RTSPTRANSPORTS=tcp \
-e MTX_WEBRTCADDITIONALHOSTS=192.168.x.x \
-p 8554:8554 \
-p 1935:1935 \
-p 8888:8888 \
-p 8889:8889 \
-p 8890:8890/udp \
-p 8189:8189/udp \
bluenviron/mediamtx
### Running the Frontend

```bash
cd frontend
npm install
npm run dev
```

The frontend will start on http://localhost:3000

## API Endpoints

### User Management
- POST /api/users/register - Register a new user
- POST /api/users/login - Login a user

### Stream Management
- GET /api/streams - List all active streams
- GET /api/streams/{id} - Get details about a specific stream
- POST /api/streams - Create a new stream (requires authentication)
- PUT /api/streams/{id} - Update stream details (requires authentication)

### Follow System
- POST /api/follows - Follow a channel
- DELETE /api/follows/{id} - Unfollow a channel
- GET /api/users/{id}/followers - Get a user's followers
- GET /api/users/{id}/following - Get channels a user follows

## System Architecture Diagram

```
┌────────────┐         ┌────────────┐         ┌───────────┐
│            │         │            │         │           │
│  Streamer  ├────────►│  RTMP      │─────────►  Transcode│
│            │         │  Ingest    │         │  Service  │
└────────────┘         └────────────┘         └─────┬─────┘
                                                    │
                                                    ▼
┌────────────┐         ┌────────────┐         ┌───────────┐
│            │         │            │         │           │
│  Viewer    │◄────────┤    CDN     │◄─────────┤    S3     │
│            │         │            │         │  Storage  │
└────────────┘         └────────────┘         └───────────┘
```

## Development Roadmap

- [ ] Chat system implementation
- [ ] Stream notifications
- [ ] Stream analytics dashboard
- [ ] Mobile responsive design
- [ ] Category browsing