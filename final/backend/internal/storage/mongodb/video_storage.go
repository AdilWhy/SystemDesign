package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"videostreaming/internal/service/video"
	pb "videostreaming/proto/video"
)

// VideoDocument represents a video document in MongoDB
type VideoDocument struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	VideoID        string             `bson:"video_id"`
	Title          string             `bson:"title"`
	Description    string             `bson:"description"`
	UserID         string             `bson:"user_id"`
	ThumbnailURL   string             `bson:"thumbnail_url"`
	VideoURL       string             `bson:"video_url"`
	DurationSeconds int64              `bson:"duration_seconds"`
	ViewCount      int64              `bson:"view_count"`
	Status         int32              `bson:"status"`
	CreatedAt      time.Time          `bson:"created_at"`
	UpdatedAt      time.Time          `bson:"updated_at"`
	Tags           []string           `bson:"tags"`
	Visibility     int32              `bson:"visibility"`
	Resolution     int32              `bson:"resolution"`
}

// StreamKeyDocument represents a stream key document in MongoDB
type StreamKeyDocument struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    string             `bson:"user_id"`
	StreamKey string             `bson:"stream_key"`
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`
}

// LiveStreamDocument represents a live stream document in MongoDB
type LiveStreamDocument struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	StreamID    string             `bson:"stream_id"`
	UserID      string             `bson:"user_id"`
	Title       string             `bson:"title"`
	Description string             `bson:"description"`
	ThumbnailURL string            `bson:"thumbnail_url"`
	PlaybackURL string             `bson:"playback_url"`
	ViewerCount int64              `bson:"viewer_count"`
	IsActive    bool               `bson:"is_active"`
	StartedAt   time.Time          `bson:"started_at"`
	EndedAt     *time.Time         `bson:"ended_at,omitempty"`
	Tags        []string           `bson:"tags"`
}

// VideoStorage implements the video.Storage interface using MongoDB
type VideoStorage struct {
	client              *mongo.Client
	database            string
	videosCollection    string
	streamKeysCollection string
	liveStreamsCollection string
}

// NewVideoStorage creates a new MongoDB-based video storage
func NewVideoStorage(client *mongo.Client, database string) *VideoStorage {
	return &VideoStorage{
		client:              client,
		database:            database,
		videosCollection:    "videos",
		streamKeysCollection: "stream_keys",
		liveStreamsCollection: "live_streams",
	}
}

// SaveVideo saves a video to MongoDB
func (s *VideoStorage) SaveVideo(ctx context.Context, video *video.Video) error {
	collection := s.client.Database(s.database).Collection(s.videosCollection)
	
	filter := bson.M{"video_id": video.ID}
	
	videoDoc := s.toVideoDocument(video)
	update := bson.M{"$set": videoDoc}
	
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to save video: %w", err)
	}
	
	return nil
}

// GetVideo retrieves a video from MongoDB by ID
func (s *VideoStorage) GetVideo(ctx context.Context, id string) (*video.Video, error) {
	collection := s.client.Database(s.database).Collection(s.videosCollection)
	
	filter := bson.M{"video_id": id}
	
	var videoDoc VideoDocument
	err := collection.FindOne(ctx, filter).Decode(&videoDoc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("video not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get video: %w", err)
	}
	
	return s.fromVideoDocument(&videoDoc), nil
}

// ListVideos retrieves a list of videos from MongoDB
func (s *VideoStorage) ListVideos(ctx context.Context, userID string, limit int, offset int) ([]*video.Video, int, error) {
	collection := s.client.Database(s.database).Collection(s.videosCollection)
	
	filter := bson.M{}
	if userID != "" {
		filter["user_id"] = userID
	}
	
	// Count total videos matching filter
	total, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count videos: %w", err)
	}
	
	findOptions := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(offset)).
		SetSort(bson.M{"created_at": -1}) // Sort by creation date, newest first
	
	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list videos: %w", err)
	}
	defer cursor.Close(ctx)
	
	var videoDocs []VideoDocument
	if err := cursor.All(ctx, &videoDocs); err != nil {
		return nil, 0, fmt.Errorf("failed to decode videos: %w", err)
	}
	
	videos := make([]*video.Video, 0, len(videoDocs))
	for _, doc := range videoDocs {
		videos = append(videos, s.fromVideoDocument(&doc))
	}
	
	return videos, int(total), nil
}

// DeleteVideo removes a video from MongoDB
func (s *VideoStorage) DeleteVideo(ctx context.Context, id string, userID string) error {
	collection := s.client.Database(s.database).Collection(s.videosCollection)
	
	filter := bson.M{
		"video_id": id,
	}
	
	// If userID is provided, use it for authorization
	if userID != "" {
		filter["user_id"] = userID
	}
	
	result, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete video: %w", err)
	}
	
	if result.DeletedCount == 0 {
		return fmt.Errorf("video not found or not authorized to delete")
	}
	
	return nil
}

// SaveStreamKey saves a stream key to MongoDB
func (s *VideoStorage) SaveStreamKey(ctx context.Context, userID string, streamKey string) error {
	collection := s.client.Database(s.database).Collection(s.streamKeysCollection)
	
	now := time.Now()
	filter := bson.M{"user_id": userID}
	update := bson.M{"$set": StreamKeyDocument{
		UserID:    userID,
		StreamKey: streamKey,
		CreatedAt: now,
		UpdatedAt: now,
	}}
	
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to save stream key: %w", err)
	}
	
	return nil
}

// GetStreamKey retrieves a stream key from MongoDB
func (s *VideoStorage) GetStreamKey(ctx context.Context, userID string) (string, error) {
	collection := s.client.Database(s.database).Collection(s.streamKeysCollection)
	
	filter := bson.M{"user_id": userID}
	
	var streamKeyDoc StreamKeyDocument
	err := collection.FindOne(ctx, filter).Decode(&streamKeyDoc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", fmt.Errorf("stream key not found: %w", err)
		}
		return "", fmt.Errorf("failed to get stream key: %w", err)
	}
	
	return streamKeyDoc.StreamKey, nil
}

// SaveLiveStream saves a live stream to MongoDB
func (s *VideoStorage) SaveLiveStream(ctx context.Context, stream *video.LiveStream) error {
	collection := s.client.Database(s.database).Collection(s.liveStreamsCollection)
	
	filter := bson.M{"stream_id": stream.StreamID}
	
	liveStreamDoc := s.toLiveStreamDocument(stream)
	update := bson.M{"$set": liveStreamDoc}
	
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to save live stream: %w", err)
	}
	
	return nil
}

// GetLiveStream retrieves a live stream from MongoDB
func (s *VideoStorage) GetLiveStream(ctx context.Context, streamID string) (*video.LiveStream, error) {
	collection := s.client.Database(s.database).Collection(s.liveStreamsCollection)
	
	filter := bson.M{"stream_id": streamID, "is_active": true}
	
	var liveStreamDoc LiveStreamDocument
	err := collection.FindOne(ctx, filter).Decode(&liveStreamDoc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("live stream not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get live stream: %w", err)
	}
	
	return s.fromLiveStreamDocument(&liveStreamDoc), nil
}

// EndLiveStream marks a live stream as ended in MongoDB
func (s *VideoStorage) EndLiveStream(ctx context.Context, streamID string, userID string) error {
	collection := s.client.Database(s.database).Collection(s.liveStreamsCollection)
	
	filter := bson.M{
		"stream_id": streamID,
		"user_id":   userID,
		"is_active": true,
	}
	
	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"is_active": false,
			"ended_at": now,
		},
	}
	
	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to end live stream: %w", err)
	}
	
	if result.MatchedCount == 0 {
		return fmt.Errorf("live stream not found or not active")
	}
	
	return nil
}

// ListLiveStreams retrieves a list of active live streams from MongoDB
func (s *VideoStorage) ListLiveStreams(ctx context.Context, userID string, limit int, offset int) ([]*video.LiveStream, int, error) {
	collection := s.client.Database(s.database).Collection(s.liveStreamsCollection)
	
	filter := bson.M{"is_active": true}
	if userID != "" {
		filter["user_id"] = userID
	}
	
	// Count total active live streams matching filter
	total, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count live streams: %w", err)
	}
	
	findOptions := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(offset)).
		SetSort(bson.M{"started_at": -1}) // Sort by start time, newest first
	
	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list live streams: %w", err)
	}
	defer cursor.Close(ctx)
	
	var liveStreamDocs []LiveStreamDocument
	if err := cursor.All(ctx, &liveStreamDocs); err != nil {
		return nil, 0, fmt.Errorf("failed to decode live streams: %w", err)
	}
	
	streams := make([]*video.LiveStream, 0, len(liveStreamDocs))
	for _, doc := range liveStreamDocs {
		streams = append(streams, s.fromLiveStreamDocument(&doc))
	}
	
	return streams, int(total), nil
}

// Helper function to convert internal video.Video to VideoDocument
func (s *VideoStorage) toVideoDocument(v *video.Video) VideoDocument {
	return VideoDocument{
		VideoID:        v.ID,
		Title:          v.Title,
		Description:    v.Description,
		UserID:         v.UserID,
		ThumbnailURL:   v.ThumbnailURL,
		VideoURL:       v.VideoURL,
		DurationSeconds: v.DurationSeconds,
		ViewCount:      v.ViewCount,
		Status:         int32(v.Status),
		CreatedAt:      v.CreatedAt,
		UpdatedAt:      v.UpdatedAt,
		Tags:           v.Tags,
		Visibility:     int32(v.Visibility),
		Resolution:     int32(v.Resolution),
	}
}

// Helper function to convert VideoDocument to internal video.Video
func (s *VideoStorage) fromVideoDocument(doc *VideoDocument) *video.Video {
	return &video.Video{
		ID:             doc.VideoID,
		Title:          doc.Title,
		Description:    doc.Description,
		UserID:         doc.UserID,
		ThumbnailURL:   doc.ThumbnailURL,
		VideoURL:       doc.VideoURL,
		DurationSeconds: doc.DurationSeconds,
		ViewCount:      doc.ViewCount,
		Status:         pb.VideoStatus(doc.Status),
		CreatedAt:      doc.CreatedAt,
		UpdatedAt:      doc.UpdatedAt,
		Tags:           doc.Tags,
		Visibility:     pb.VideoVisibility(doc.Visibility),
		Resolution:     pb.VideoResolution(doc.Resolution),
	}
}

// Helper function to convert internal video.LiveStream to LiveStreamDocument
func (s *VideoStorage) toLiveStreamDocument(ls *video.LiveStream) LiveStreamDocument {
	return LiveStreamDocument{
		StreamID:     ls.StreamID,
		UserID:       ls.UserID,
		Title:        ls.Title,
		Description:  ls.Description,
		ThumbnailURL: ls.ThumbnailURL,
		PlaybackURL:  ls.PlaybackURL,
		ViewerCount:  ls.ViewerCount,
		IsActive:     true,
		StartedAt:    ls.StartedAt,
		EndedAt:      nil,
		Tags:         ls.Tags,
	}
}

// Helper function to convert LiveStreamDocument to internal video.LiveStream
func (s *VideoStorage) fromLiveStreamDocument(doc *LiveStreamDocument) *video.LiveStream {
	return &video.LiveStream{
		StreamID:     doc.StreamID,
		UserID:       doc.UserID,
		Title:        doc.Title,
		Description:  doc.Description,
		ThumbnailURL: doc.ThumbnailURL,
		PlaybackURL:  doc.PlaybackURL,
		ViewerCount:  doc.ViewerCount,
		StartedAt:    doc.StartedAt,
		Tags:         doc.Tags,
	}
}