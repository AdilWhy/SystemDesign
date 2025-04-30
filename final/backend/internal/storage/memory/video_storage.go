package memory

import (
	"context"
	"fmt"
	"sync"

	"videostreaming/internal/service/video"
)

// VideoStorage implements the video.Storage interface using in-memory maps
type VideoStorage struct {
	videos      map[string]*video.Video
	streamKeys  map[string]string
	liveStreams map[string]*video.LiveStream
	mutex       sync.RWMutex
}

// NewVideoStorage creates a new in-memory video storage
func NewVideoStorage() *VideoStorage {
	return &VideoStorage{
		videos:      make(map[string]*video.Video),
		streamKeys:  make(map[string]string),
		liveStreams: make(map[string]*video.LiveStream),
	}
}

// SaveVideo saves a video to memory
func (s *VideoStorage) SaveVideo(ctx context.Context, v *video.Video) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.videos[v.ID] = v
	return nil
}

// GetVideo retrieves a video from memory
func (s *VideoStorage) GetVideo(ctx context.Context, id string) (*video.Video, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	v, exists := s.videos[id]
	if !exists {
		return nil, fmt.Errorf("video not found: %s", id)
	}
	
	return v, nil
}

// ListVideos retrieves a list of videos from memory
func (s *VideoStorage) ListVideos(ctx context.Context, userID string, limit int, offset int) ([]*video.Video, int, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	var videos []*video.Video
	var count int
	
	// If userID is provided, filter by user
	if userID != "" {
		for _, v := range s.videos {
			if v.UserID == userID {
				videos = append(videos, v)
			}
		}
	} else {
		// Otherwise, collect all videos
		for _, v := range s.videos {
			videos = append(videos, v)
		}
	}
	
	// Count total matching videos
	count = len(videos)
	
	// Apply pagination
	if offset >= len(videos) {
		return []*video.Video{}, count, nil
	}
	
	end := offset + limit
	if end > len(videos) {
		end = len(videos)
	}
	
	return videos[offset:end], count, nil
}

// DeleteVideo removes a video from memory
func (s *VideoStorage) DeleteVideo(ctx context.Context, id string, userID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	v, exists := s.videos[id]
	if !exists {
		return fmt.Errorf("video not found: %s", id)
	}
	
	// Verify user has permission to delete
	if userID != "" && v.UserID != userID {
		return fmt.Errorf("not authorized to delete video: %s", id)
	}
	
	delete(s.videos, id)
	return nil
}

// SaveStreamKey saves a stream key to memory
func (s *VideoStorage) SaveStreamKey(ctx context.Context, userID string, streamKey string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.streamKeys[userID] = streamKey
	return nil
}

// GetStreamKey retrieves a stream key from memory
func (s *VideoStorage) GetStreamKey(ctx context.Context, userID string) (string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	streamKey, exists := s.streamKeys[userID]
	if (!exists) {
		return "", fmt.Errorf("stream key not found for user: %s", userID)
	}
	
	return streamKey, nil
}

// SaveLiveStream saves a live stream to memory
func (s *VideoStorage) SaveLiveStream(ctx context.Context, stream *video.LiveStream) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.liveStreams[stream.StreamID] = stream
	return nil
}

// GetLiveStream retrieves a live stream from memory
func (s *VideoStorage) GetLiveStream(ctx context.Context, streamID string) (*video.LiveStream, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	stream, exists := s.liveStreams[streamID]
	if (!exists) {
		return nil, fmt.Errorf("live stream not found: %s", streamID)
	}
	
	return stream, nil
}

// EndLiveStream marks a live stream as ended in memory
func (s *VideoStorage) EndLiveStream(ctx context.Context, streamID string, userID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	stream, exists := s.liveStreams[streamID]
	if (!exists) {
		return fmt.Errorf("live stream not found: %s", streamID)
	}
	
	// Verify user has permission to end the stream
	if (userID != "" && stream.UserID != userID) {
		return fmt.Errorf("not authorized to end stream: %s", streamID)
	}
	
	// In a real implementation, we would mark the stream as ended
	// For simplicity, we'll just remove it from the active streams
	delete(s.liveStreams, streamID)
	
	return nil
}

// ListLiveStreams retrieves a list of active live streams from memory
func (s *VideoStorage) ListLiveStreams(ctx context.Context, userID string, limit int, offset int) ([]*video.LiveStream, int, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	var streams []*video.LiveStream
	
	// If userID is provided, filter by user
	if (userID != "") {
		for _, stream := range s.liveStreams {
			if (stream.UserID == userID) {
				streams = append(streams, stream)
			}
		}
	} else {
		// Otherwise, collect all streams
		for _, stream := range s.liveStreams {
			streams = append(streams, stream)
		}
	}
	
	// Count total matching streams
	count := len(streams)
	
	// Apply pagination
	if (offset >= len(streams)) {
		return []*video.LiveStream{}, count, nil
	}
	
	end := offset + limit
	if (end > len(streams)) {
		end = len(streams)
	}
	
	return streams[offset:end], count, nil
}