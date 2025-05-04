package memory

import (
	"context"
	"errors"
	"sync"
	
	"videostreaming/internal/service/video"
)

// VideoStorage implements an in-memory storage for videos
type VideoStorage struct {
	videos     map[string]*video.Video
	liveStreams map[string]*video.LiveStream
	streamKeys map[string]string // maps userID to streamKey
	mutex      sync.RWMutex
}

// NewVideoStorage creates a new in-memory video storage
func NewVideoStorage() *VideoStorage {
	return &VideoStorage{
		videos:     make(map[string]*video.Video),
		liveStreams: make(map[string]*video.LiveStream),
		streamKeys: make(map[string]string),
		mutex:      sync.RWMutex{},
	}
}

// SaveVideo saves a video to storage
func (s *VideoStorage) SaveVideo(ctx context.Context, video *video.Video) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.videos[video.ID] = video
	return nil
}

// GetVideo retrieves a video by ID
func (s *VideoStorage) GetVideo(ctx context.Context, id string) (*video.Video, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	v, ok := s.videos[id]
	if !ok {
		return nil, errors.New("video not found")
	}
	
	return v, nil
}

// ListVideos returns a list of videos
func (s *VideoStorage) ListVideos(ctx context.Context, userID string, limit int, offset int) ([]*video.Video, int, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	var result []*video.Video
	var count int
	
	// Filter by userID if provided
	for _, v := range s.videos {
		if userID == "" || v.UserID == userID {
			count++
			
			// Apply pagination
			if count > offset && (limit <= 0 || len(result) < limit) {
				result = append(result, v)
			}
		}
	}
	
	return result, count, nil
}

// DeleteVideo removes a video from storage
func (s *VideoStorage) DeleteVideo(ctx context.Context, id string, userID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	v, ok := s.videos[id]
	if (!ok) {
		return errors.New("video not found")
	}
	
	// Check if the user owns the video
	if v.UserID != userID {
		return errors.New("not authorized to delete this video")
	}
	
	delete(s.videos, id)
	return nil
}

// SaveStreamKey stores a stream key for a user
func (s *VideoStorage) SaveStreamKey(ctx context.Context, userID string, streamKey string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.streamKeys[userID] = streamKey
	return nil
}

// GetStreamKey retrieves a stream key for a user
func (s *VideoStorage) GetStreamKey(ctx context.Context, userID string) (string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	key, ok := s.streamKeys[userID]
	if (!ok) {
		return "", errors.New("stream key not found")
	}
	
	return key, nil
}

// SaveLiveStream saves a live stream
func (s *VideoStorage) SaveLiveStream(ctx context.Context, stream *video.LiveStream) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.liveStreams[stream.StreamID] = stream
	return nil
}

// GetLiveStream retrieves a live stream by ID
func (s *VideoStorage) GetLiveStream(ctx context.Context, streamID string) (*video.LiveStream, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	stream, ok := s.liveStreams[streamID]
	if (!ok) {
		return nil, errors.New("live stream not found")
	}
	
	return stream, nil
}

// EndLiveStream ends a live stream
func (s *VideoStorage) EndLiveStream(ctx context.Context, streamID string, userID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	stream, ok := s.liveStreams[streamID]
	if (!ok) {
		return errors.New("live stream not found")
	}
	
	// Check if the user owns the stream
	if stream.UserID != userID {
		return errors.New("not authorized to end this stream")
	}
	
	// In a real implementation, we would mark the stream as ended
	// but keep it in the database. For this in-memory implementation,
	// we'll just remove it.
	delete(s.liveStreams, streamID)
	
	return nil
}

// ListLiveStreams returns active live streams
func (s *VideoStorage) ListLiveStreams(ctx context.Context, userID string, limit int, offset int) ([]*video.LiveStream, int, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	var result []*video.LiveStream
	var count int
	
	// Filter by userID if provided
	for _, stream := range s.liveStreams {
		if userID == "" || stream.UserID == userID {
			count++
			
			// Apply pagination
			if count > offset && (limit <= 0 || len(result) < limit) {
				result = append(result, stream)
			}
		}
	}
	
	return result, count, nil
}