package video

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "videostreaming/proto/video"
)

// Storage defines the interface for video data persistence
type Storage interface {
	SaveVideo(ctx context.Context, video *Video) error
	GetVideo(ctx context.Context, id string) (*Video, error)
	ListVideos(ctx context.Context, userID string, limit int, offset int) ([]*Video, int, error)
	DeleteVideo(ctx context.Context, id string, userID string) error
	
	// Live streaming methods
	SaveStreamKey(ctx context.Context, userID string, streamKey string) error
	GetStreamKey(ctx context.Context, userID string) (string, error)
	SaveLiveStream(ctx context.Context, stream *LiveStream) error
	GetLiveStream(ctx context.Context, streamID string) (*LiveStream, error)
	EndLiveStream(ctx context.Context, streamID string, userID string) error
	ListLiveStreams(ctx context.Context, userID string, limit int, offset int) ([]*LiveStream, int, error)
}

// BlobStorage defines the interface for file storage operations
type BlobStorage interface {
	GenerateUploadURL(ctx context.Context, key string, contentType string, expiresIn time.Duration) (string, error)
	GenerateDownloadURL(ctx context.Context, key string, expiresIn time.Duration) (string, error)
	DeleteObject(ctx context.Context, key string) error
}

// TranscodingService defines the interface for video transcoding operations
type TranscodingService interface {
	StartTranscoding(ctx context.Context, videoID string, inputPath string) error
	GetTranscodingStatus(ctx context.Context, videoID string) (*TranscodingStatus, error)
}

// StreamingEngine defines the interface for live streaming operations
type StreamingEngine interface {
	GenerateStreamKey(ctx context.Context, userID string) (string, error)
	GetRTMPURL() string
	GetStreamPlaybackURL(streamID string) string
}

// Video represents a video in the system
type Video struct {
	ID             string
	Title          string
	Description    string
	UserID         string
	ThumbnailURL   string
	VideoURL       string
	DurationSeconds int64
	ViewCount      int64
	Status         pb.VideoStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Tags           []string
	Visibility     pb.VideoVisibility
	Resolution     pb.VideoResolution
}

// LiveStream represents an active live stream
type LiveStream struct {
	StreamID      string
	UserID        string
	Title         string
	Description   string
	ThumbnailURL  string
	PlaybackURL   string
	ViewerCount   int64
	StartedAt     time.Time
	Tags          []string
}

// TranscodingStatus represents the status of a video transcoding job
type TranscodingStatus struct {
	VideoID         string
	Status          pb.TranscodingStatus
	Jobs            []*TranscodingJob
	OverallProgress float32
}

// TranscodingJob represents a single resolution transcoding job
type TranscodingJob struct {
	JobID       string
	Resolution  pb.VideoResolution
	Status      pb.TranscodingStatus
	Progress    float32
	ErrorMessage string
}

// Service implements the video service
type Service struct {
	storage          Storage
	blobStorage      BlobStorage
	transcodingService TranscodingService
	streamingEngine  StreamingEngine
	
	// Configuration
	uploadExpiry     time.Duration
	downloadExpiry   time.Duration
	videoKeyPrefix   string
	thumbnailKeyPrefix string
	rtmpURL          string
	pb.UnimplementedVideoServiceServer
}

// NewService creates a new video service
func NewService(storage Storage, blobStorage BlobStorage, transcodingService TranscodingService, streamingEngine StreamingEngine) *Service {
	return &Service{
		storage:          storage,
		blobStorage:      blobStorage,
		transcodingService: transcodingService,
		streamingEngine:  streamingEngine,
		uploadExpiry:     time.Hour,
		downloadExpiry:   time.Hour * 24,
		videoKeyPrefix:   "videos/",
		thumbnailKeyPrefix: "thumbnails/",
	}
}

// InitiateUpload handles the request to start a video upload
func (s *Service) InitiateUpload(ctx context.Context, req *pb.InitiateUploadRequest) (*pb.InitiateUploadResponse, error) {
	videoID := uuid.New().String()
	uploadID := uuid.New().String()
	
	video := &Video{
		ID:          videoID,
		Title:       req.Title,
		Description: req.Description,
		UserID:      req.UserId,
		Status:      pb.VideoStatus_VIDEO_STATUS_UPLOADING,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Tags:        req.Tags,
		Visibility:  req.Visibility,
	}
	
	// Save initial video metadata
	if err := s.storage.SaveVideo(ctx, video); err != nil {
		return nil, fmt.Errorf("failed to save video metadata: %w", err)
	}
	
	// Generate signed upload URL
	objectKey := s.videoKeyPrefix + videoID
	uploadURL, err := s.blobStorage.GenerateUploadURL(ctx, objectKey, req.ContentType, s.uploadExpiry)
	if err != nil {
		return nil, fmt.Errorf("failed to generate upload URL: %w", err)
	}
	
	return &pb.InitiateUploadResponse{
		UploadId: uploadID,
		VideoId:  videoID,
		UploadUrl: uploadURL,
	}, nil
}

// CompleteUpload handles the request to finalize a video upload
func (s *Service) CompleteUpload(ctx context.Context, req *pb.CompleteUploadRequest) (*pb.CompleteUploadResponse, error) {
	video, err := s.storage.GetVideo(ctx, req.VideoId)
	if err != nil {
		return nil, fmt.Errorf("failed to get video: %w", err)
	}
	
	// Update video status to processing
	video.Status = pb.VideoStatus_VIDEO_STATUS_PROCESSING
	video.UpdatedAt = time.Now()
	
	if err := s.storage.SaveVideo(ctx, video); err != nil {
		return nil, fmt.Errorf("failed to update video status: %w", err)
	}
	
	// Start transcoding process
	objectKey := s.videoKeyPrefix + req.VideoId
	if err := s.transcodingService.StartTranscoding(ctx, req.VideoId, objectKey); err != nil {
		return nil, fmt.Errorf("failed to start transcoding: %w", err)
	}
	
	return &pb.CompleteUploadResponse{
		VideoId: req.VideoId,
		Status:  pb.VideoStatus_VIDEO_STATUS_PROCESSING,
	}, nil
}

// GetVideo retrieves video metadata
func (s *Service) GetVideo(ctx context.Context, req *pb.GetVideoRequest) (*pb.Video, error) {
	video, err := s.storage.GetVideo(ctx, req.VideoId)
	if err != nil {
		return nil, fmt.Errorf("failed to get video: %w", err)
	}
	
	// Generate signed URL for the video if it's ready
	if video.Status == pb.VideoStatus_VIDEO_STATUS_READY {
		objectKey := s.videoKeyPrefix + video.ID
		videoURL, err := s.blobStorage.GenerateDownloadURL(ctx, objectKey, s.downloadExpiry)
		if err != nil {
			return nil, fmt.Errorf("failed to generate download URL: %w", err)
		}
		video.VideoURL = videoURL
	}
	
	return toProtoVideo(video), nil
}

// ListVideos retrieves a list of videos
func (s *Service) ListVideos(ctx context.Context, req *pb.ListVideosRequest) (*pb.ListVideosResponse, error) {
	// Parse pagination
	limit := int(req.PageSize)
	if limit <= 0 {
		limit = 20 // Default page size
	}
	
	offset := 0
	if req.PageToken != "" {
		// In a real implementation, we would decode the page token to get the offset
		// For simplicity, we're using a simple string here
		// In production, use a proper pagination token scheme
	}
	
	videos, total, err := s.storage.ListVideos(ctx, req.UserId, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list videos: %w", err)
	}
	
	protoVideos := make([]*pb.Video, 0, len(videos))
	for _, video := range videos {
		protoVideos = append(protoVideos, toProtoVideo(video))
	}
	
	nextPageToken := ""
	if len(videos) == limit {
		// Generate next page token
		nextPageToken = fmt.Sprintf("%d", offset+limit)
	}
	
	return &pb.ListVideosResponse{
		Videos:        protoVideos,
		NextPageToken: nextPageToken,
		TotalCount:    int32(total),
	}, nil
}

// DeleteVideo removes a video
func (s *Service) DeleteVideo(ctx context.Context, req *pb.DeleteVideoRequest) (*emptypb.Empty, error) {
	if err := s.storage.DeleteVideo(ctx, req.VideoId, req.UserId); err != nil {
		return nil, fmt.Errorf("failed to delete video from database: %w", err)
	}
	
	// Delete the video file from blob storage
	objectKey := s.videoKeyPrefix + req.VideoId
	if err := s.blobStorage.DeleteObject(ctx, objectKey); err != nil {
		// Log the error but don't fail the request
		fmt.Printf("failed to delete video file from blob storage: %v", err)
	}
	
	// Delete thumbnail if exists
	thumbnailKey := s.thumbnailKeyPrefix + req.VideoId
	if err := s.blobStorage.DeleteObject(ctx, thumbnailKey); err != nil {
		// Log the error but don't fail the request
		fmt.Printf("failed to delete thumbnail from blob storage: %v", err)
	}
	
	return &emptypb.Empty{}, nil
}

// GetStreamKey retrieves or creates a streaming key for a user
func (s *Service) GetStreamKey(ctx context.Context, req *pb.GetStreamKeyRequest) (*pb.StreamKeyResponse, error) {
	// Try to get existing stream key
	streamKey, err := s.storage.GetStreamKey(ctx, req.UserId)
	if err != nil {
		// Generate a new stream key
		streamKey, err = s.streamingEngine.GenerateStreamKey(ctx, req.UserId)
		if err != nil {
			return nil, fmt.Errorf("failed to generate stream key: %w", err)
		}
		
		// Save the stream key
		if err := s.storage.SaveStreamKey(ctx, req.UserId, streamKey); err != nil {
			return nil, fmt.Errorf("failed to save stream key: %w", err)
		}
	}
	
	return &pb.StreamKeyResponse{
		StreamKey: streamKey,
		RtmpUrl:   s.streamingEngine.GetRTMPURL(),
	}, nil
}

// StartStream begins a new live stream
func (s *Service) StartStream(ctx context.Context, req *pb.StartStreamRequest) (*pb.StreamResponse, error) {
	// Verify the stream key
	existingKey, err := s.storage.GetStreamKey(ctx, req.UserId)
	if err != nil || existingKey != req.StreamKey {
		return nil, fmt.Errorf("invalid stream key")
	}
	
	streamID := uuid.New().String()
	
	liveStream := &LiveStream{
		StreamID:    streamID,
		UserID:      req.UserId,
		Title:       req.Title,
		Description: req.Description,
		PlaybackURL: s.streamingEngine.GetStreamPlaybackURL(streamID),
		StartedAt:   time.Now(),
		Tags:        req.Tags,
	}
	
	if err := s.storage.SaveLiveStream(ctx, liveStream); err != nil {
		return nil, fmt.Errorf("failed to save live stream: %w", err)
	}
	
	return &pb.StreamResponse{
		StreamId:    streamID,
		PlaybackUrl: liveStream.PlaybackURL,
		StreamKey:   req.StreamKey,
	}, nil
}

// EndStream terminates a live stream
func (s *Service) EndStream(ctx context.Context, req *pb.EndStreamRequest) (*emptypb.Empty, error) {
	if err := s.storage.EndLiveStream(ctx, req.StreamId, req.UserId); err != nil {
		return nil, fmt.Errorf("failed to end live stream: %w", err)
	}
	
	return &emptypb.Empty{}, nil
}

// GetLiveStreams retrieves active live streams
func (s *Service) GetLiveStreams(ctx context.Context, req *pb.GetLiveStreamsRequest) (*pb.GetLiveStreamsResponse, error) {
	// Parse pagination
	limit := int(req.PageSize)
	if limit <= 0 {
		limit = 20 // Default page size
	}
	
	offset := 0
	if req.PageToken != "" {
		// In a real implementation, we would decode the page token to get the offset
	}
	
	streams, total, err := s.storage.ListLiveStreams(ctx, req.UserId, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list live streams: %w", err)
	}
	
	protoStreams := make([]*pb.LiveStream, 0, len(streams))
	for _, stream := range streams {
		protoStreams = append(protoStreams, &pb.LiveStream{
			StreamId:     stream.StreamID,
			UserId:       stream.UserID,
			Title:        stream.Title,
			Description:  stream.Description,
			ThumbnailUrl: stream.ThumbnailURL,
			PlaybackUrl:  stream.PlaybackURL,
			ViewerCount:  stream.ViewerCount,
			StartedAt:    timestamppb.New(stream.StartedAt),
			Tags:         stream.Tags,
		})
	}
	
	nextPageToken := ""
	if len(streams) == limit {
		nextPageToken = fmt.Sprintf("%d", offset+limit)
	}
	
	return &pb.GetLiveStreamsResponse{
		Streams:       protoStreams,
		NextPageToken: nextPageToken,
		TotalCount:    int32(total),
	}, nil
}

// GetTranscodingStatus retrieves the status of video transcoding
func (s *Service) GetTranscodingStatus(ctx context.Context, req *pb.GetTranscodingStatusRequest) (*pb.TranscodingStatusResponse, error) {
	status, err := s.transcodingService.GetTranscodingStatus(ctx, req.VideoId)
	if err != nil {
		return nil, fmt.Errorf("failed to get transcoding status: %w", err)
	}
	
	jobs := make([]*pb.TranscodingJob, 0, len(status.Jobs))
	for _, job := range status.Jobs {
		jobs = append(jobs, &pb.TranscodingJob{
			JobId:        job.JobID,
			Resolution:   job.Resolution,
			Status:       job.Status,
			Progress:     job.Progress,
			ErrorMessage: job.ErrorMessage,
		})
	}
	
	return &pb.TranscodingStatusResponse{
		VideoId:        status.VideoID,
		Status:         status.Status,
		Jobs:           jobs,
		OverallProgress: status.OverallProgress,
	}, nil
}

// Helper function to convert internal Video type to proto
func toProtoVideo(v *Video) *pb.Video {
	return &pb.Video{
		Id:             v.ID,
		Title:          v.Title,
		Description:    v.Description,
		UserId:         v.UserID,
		ThumbnailUrl:   v.ThumbnailURL,
		VideoUrl:       v.VideoURL,
		DurationSeconds: v.DurationSeconds,
		ViewCount:      v.ViewCount,
		Status:         v.Status,
		CreatedAt:      timestamppb.New(v.CreatedAt),
		UpdatedAt:      timestamppb.New(v.UpdatedAt),
		Tags:           v.Tags,
		Visibility:     v.Visibility,
		Resolution:     v.Resolution,
	}
}