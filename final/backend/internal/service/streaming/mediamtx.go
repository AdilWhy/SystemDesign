package streaming

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// MediaMTXEngine implements the StreamingEngine interface using MediaMTX server
type MediaMTXEngine struct {
	rtmpServerURL  string
	hlsServerURL   string
	webRTCServerURL string
}

// NewMediaMTXEngine creates a new MediaMTX streaming engine
func NewMediaMTXEngine(rtmpServerURL, hlsServerURL, webRTCServerURL string) *MediaMTXEngine {
	return &MediaMTXEngine{
		rtmpServerURL:  rtmpServerURL,
		hlsServerURL:   hlsServerURL,
		webRTCServerURL: webRTCServerURL,
	}
}

// GenerateStreamKey creates a unique stream key for a user
func (e *MediaMTXEngine) GenerateStreamKey(ctx context.Context, userID string) (string, error) {
	// Generate a random key with a prefix of the user ID to make it unique
	// In production, you might want to store this in a database and check for uniqueness
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random stream key: %w", err)
	}
	
	// Use a safe prefix from userID (handle cases where userID length < 8)
	prefix := userID
	if len(userID) > 8 {
		prefix = userID[:8]
	}
	
	streamKey := fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(randomBytes))
	return streamKey, nil
}

// GetRTMPURL returns the RTMP URL for streaming
func (e *MediaMTXEngine) GetRTMPURL() string {
	return e.rtmpServerURL
}

// GetStreamPlaybackURL returns the playback URL for a stream
// For MediaMTX, this is typically the HLS URL with the stream key
func (e *MediaMTXEngine) GetStreamPlaybackURL(streamID string) string {
	// In MediaMTX, the stream name in the URL is typically the stream key
	// The actual playback URL will depend on how your frontend consumes the stream
	return fmt.Sprintf("%s/%s/index.m3u8", e.hlsServerURL, streamID)
}

// GetWebRTCPlaybackURL returns the WebRTC playback URL
func (e *MediaMTXEngine) GetWebRTCPlaybackURL(streamID string) string {
	return fmt.Sprintf("%s/%s", e.webRTCServerURL, streamID)
}

// IsStreamActive checks if a stream is currently active on the server
// This would typically involve calling an API on the MediaMTX server
// or checking a status endpoint. For simplicity, we're returning true here.
func (e *MediaMTXEngine) IsStreamActive(streamID string) bool {
	// In a real implementation, you would check if the stream is active on the MediaMTX server
	// This could involve making an API call to MediaMTX's API endpoint
	// For now, we'll just return true for demonstration purposes
	return true
}