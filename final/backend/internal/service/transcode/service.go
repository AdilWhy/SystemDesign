package transcode

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	pb "videostreaming/proto/video"
)

// TranscodeStorage defines the interface for transcoding job storage
type TranscodeStorage interface {
	SaveTranscodingJob(ctx context.Context, job *TranscodingJob) error
	GetTranscodingJobs(ctx context.Context, videoID string) ([]*TranscodingJob, error)
	UpdateTranscodingJob(ctx context.Context, job *TranscodingJob) error
}

// FFmpegClient defines the interface for FFmpeg operations
type FFmpegClient interface {
	TranscodeVideo(ctx context.Context, inputPath string, outputPath string, options TranscodeOptions) error
	GetMediaInfo(ctx context.Context, filePath string) (*MediaInfo, error)
}

// S3Storage defines the interface for S3 storage operations
type S3Storage interface {
	GenerateUploadURL(ctx context.Context, key string, contentType string, expiresIn time.Duration) (string, error)
	GenerateDownloadURL(ctx context.Context, key string, expiresIn time.Duration) (string, error)
	DeleteObject(ctx context.Context, key string) error
}

// NotificationService defines the interface for notifications
type NotificationService interface {
	NotifyTranscodingComplete(ctx context.Context, videoID string, status pb.TranscodingStatus) error
	NotifyTranscodingProgress(ctx context.Context, videoID string, progress float32) error
}

// TranscodingJob represents a video transcoding job
type TranscodingJob struct {
	ID             string
	VideoID        string
	InputPath      string
	OutputPath     string
	Resolution     pb.VideoResolution
	Status         pb.TranscodingStatus
	Progress       float32
	StartTime      time.Time
	CompletionTime *time.Time
	ErrorMessage   string
}

// MediaInfo contains metadata about a media file
type MediaInfo struct {
	Duration  float64
	Width     int
	Height    int
	Bitrate   int64
	Codec     string
	FrameRate float64
}

// TranscodeOptions defines options for video transcoding
type TranscodeOptions struct {
	Resolution  pb.VideoResolution
	VideoBitrate string
	AudioBitrate string
	Format       string
	Codec        string
	FrameRate    int
}

// Service handles video transcoding
type Service struct {
	storage             TranscodeStorage
	ffmpegClient        FFmpegClient
	s3Storage           S3Storage
	notificationService NotificationService

	// Configuration
	outputKeyPrefix  string
	availableFormats []string
	bitrates         map[pb.VideoResolution]string
	audioBitrate     string
	codec            string

	// State
	jobs     map[string]*jobState
	jobsLock sync.RWMutex
}

type jobState struct {
	videoID   string
	jobs      []*TranscodingJob
	progress  float32
	status    pb.TranscodingStatus
	updatedAt time.Time
}

// NewService creates a new transcoding service
func NewService(
	storage TranscodeStorage,
	ffmpegClient FFmpegClient,
	s3Storage S3Storage,
	notificationService NotificationService,
) *Service {
	return &Service{
		storage:             storage,
		ffmpegClient:        ffmpegClient,
		s3Storage:           s3Storage,
		notificationService: notificationService,
		outputKeyPrefix:     "transcoded/",
		availableFormats:    []string{"hls", "mp4"},
		bitrates: map[pb.VideoResolution]string{
			pb.VideoResolution_VIDEO_RESOLUTION_240P:  "500k",
			pb.VideoResolution_VIDEO_RESOLUTION_360P:  "800k",
			pb.VideoResolution_VIDEO_RESOLUTION_480P:  "1500k",
			pb.VideoResolution_VIDEO_RESOLUTION_720P:  "3000k",
			pb.VideoResolution_VIDEO_RESOLUTION_1080P: "5000k",
			pb.VideoResolution_VIDEO_RESOLUTION_1440P: "8000k",
			pb.VideoResolution_VIDEO_RESOLUTION_2160P: "16000k",
		},
		audioBitrate: "128k",
		codec:        "libx264",
		jobs:         make(map[string]*jobState),
	}
}

// StartTranscoding begins the transcoding process for a video
func (s *Service) StartTranscoding(ctx context.Context, videoID string, inputPath string) error {
	// Get media info to determine appropriate transcoding parameters
	mediaInfo, err := s.ffmpegClient.GetMediaInfo(ctx, inputPath)
	if err != nil {
		return fmt.Errorf("failed to get media info: %w", err)
	}

	// Create transcoding jobs for different resolutions
	resolutions := s.determineTargetResolutions(mediaInfo.Width, mediaInfo.Height)

	for _, resolution := range resolutions {
		jobID := uuid.New().String()
		outputPath := fmt.Sprintf("%s%s/%s", s.outputKeyPrefix, videoID, s.getResolutionPath(resolution))

		job := &TranscodingJob{
			ID:         jobID,
			VideoID:    videoID,
			InputPath:  inputPath,
			OutputPath: outputPath,
			Resolution: resolution,
			Status:     pb.TranscodingStatus_TRANSCODING_STATUS_QUEUED,
			Progress:   0,
			StartTime:  time.Now(),
		}

		// Save the job to storage
		if err := s.storage.SaveTranscodingJob(ctx, job); err != nil {
			return fmt.Errorf("failed to save transcoding job: %w", err)
		}

		// Start transcoding in a goroutine
		go s.processTranscoding(context.Background(), job)
	}

	return nil
}

// GetTranscodingStatus returns the current status of the transcoding jobs for a video
func (s *Service) GetTranscodingStatus(ctx context.Context, videoID string) (*pb.TranscodingStatusResponse, error) {
	jobs, err := s.storage.GetTranscodingJobs(ctx, videoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transcoding jobs: %w", err)
	}

	if len(jobs) == 0 {
		return &pb.TranscodingStatusResponse{
			VideoId: videoID,
			Status:  pb.TranscodingStatus_TRANSCODING_STATUS_NOT_FOUND,
		}, nil
	}

	// Calculate overall status and progress
	var totalProgress float32
	var errorCount int
	var completedCount int

	protoJobs := make([]*pb.TranscodingJob, len(jobs))

	for i, job := range jobs {
		totalProgress += job.Progress

		if job.Status == pb.TranscodingStatus_TRANSCODING_STATUS_ERROR {
			errorCount++
		} else if job.Status == pb.TranscodingStatus_TRANSCODING_STATUS_COMPLETED {
			completedCount++
		}

		protoJobs[i] = &pb.TranscodingJob{
			JobId:        job.ID,
			Resolution:   job.Resolution,
			Status:       job.Status,
			Progress:     job.Progress,
			ErrorMessage: job.ErrorMessage,
		}
	}

	overallProgress := totalProgress / float32(len(jobs))
	var overallStatus pb.TranscodingStatus

	if errorCount == len(jobs) {
		overallStatus = pb.TranscodingStatus_TRANSCODING_STATUS_ERROR
	} else if completedCount == len(jobs) {
		overallStatus = pb.TranscodingStatus_TRANSCODING_STATUS_COMPLETED
	} else {
		overallStatus = pb.TranscodingStatus_TRANSCODING_STATUS_PROCESSING
	}

	return &pb.TranscodingStatusResponse{
		VideoId:         videoID,
		Status:          overallStatus,
		Jobs:            protoJobs,
		OverallProgress: overallProgress,
	}, nil
}

// processTranscoding handles the actual transcoding process for a job
func (s *Service) processTranscoding(ctx context.Context, job *TranscodingJob) {
	// Update job status to processing
	job.Status = pb.TranscodingStatus_TRANSCODING_STATUS_PROCESSING
	if err := s.storage.UpdateTranscodingJob(ctx, job); err != nil {
		log.Printf("Failed to update transcoding job status: %v", err)
		return
	}

	// Prepare transcoding options based on target resolution
	options := TranscodeOptions{
		Resolution:  job.Resolution,
		VideoBitrate: s.bitrates[job.Resolution],
		AudioBitrate: s.audioBitrate,
		Format:       "hls", // Use HLS for adaptive streaming
		Codec:        s.codec,
		FrameRate:    30,
	}

	// Start transcoding
	if err := s.ffmpegClient.TranscodeVideo(ctx, job.InputPath, job.OutputPath, options); err != nil {
		// Handle transcoding error
		job.Status = pb.TranscodingStatus_TRANSCODING_STATUS_ERROR
		job.ErrorMessage = err.Error()
		if err := s.storage.UpdateTranscodingJob(ctx, job); err != nil {
			log.Printf("Failed to update transcoding job error: %v", err)
		}

		// Notify about error
		s.notificationService.NotifyTranscodingComplete(ctx, job.VideoID, pb.TranscodingStatus_TRANSCODING_STATUS_ERROR)
		return
	}

	// Update job status to completed
	job.Status = pb.TranscodingStatus_TRANSCODING_STATUS_COMPLETED
	job.Progress = 100
	completionTime := time.Now()
	job.CompletionTime = &completionTime

	if err := s.storage.UpdateTranscodingJob(ctx, job); err != nil {
		log.Printf("Failed to update transcoding job completion: %v", err)
		return
	}

	// Notify about completion
	s.notificationService.NotifyTranscodingComplete(ctx, job.VideoID, pb.TranscodingStatus_TRANSCODING_STATUS_COMPLETED)
}

// determineTargetResolutions selects appropriate resolutions based on the source video
func (s *Service) determineTargetResolutions(width int, height int) []pb.VideoResolution {
	maxDimension := width
	if height > width {
		maxDimension = height
	}

	resolutions := []pb.VideoResolution{}

	// Add resolutions based on source video
	if maxDimension >= 3840 { // 4K UHD
		resolutions = append(resolutions,
			pb.VideoResolution_VIDEO_RESOLUTION_2160P,
			pb.VideoResolution_VIDEO_RESOLUTION_1440P,
			pb.VideoResolution_VIDEO_RESOLUTION_1080P,
			pb.VideoResolution_VIDEO_RESOLUTION_720P,
			pb.VideoResolution_VIDEO_RESOLUTION_480P,
			pb.VideoResolution_VIDEO_RESOLUTION_360P)
	} else if maxDimension >= 2560 { // 1440p
		resolutions = append(resolutions,
			pb.VideoResolution_VIDEO_RESOLUTION_1440P,
			pb.VideoResolution_VIDEO_RESOLUTION_1080P,
			pb.VideoResolution_VIDEO_RESOLUTION_720P,
			pb.VideoResolution_VIDEO_RESOLUTION_480P,
			pb.VideoResolution_VIDEO_RESOLUTION_360P)
	} else if maxDimension >= 1920 { // 1080p
		resolutions = append(resolutions,
			pb.VideoResolution_VIDEO_RESOLUTION_1080P,
			pb.VideoResolution_VIDEO_RESOLUTION_720P,
			pb.VideoResolution_VIDEO_RESOLUTION_480P,
			pb.VideoResolution_VIDEO_RESOLUTION_360P)
	} else if maxDimension >= 1280 { // 720p
		resolutions = append(resolutions,
			pb.VideoResolution_VIDEO_RESOLUTION_720P,
			pb.VideoResolution_VIDEO_RESOLUTION_480P,
			pb.VideoResolution_VIDEO_RESOLUTION_360P)
	} else if maxDimension >= 854 { // 480p
		resolutions = append(resolutions,
			pb.VideoResolution_VIDEO_RESOLUTION_480P,
			pb.VideoResolution_VIDEO_RESOLUTION_360P)
	} else {
		resolutions = append(resolutions, pb.VideoResolution_VIDEO_RESOLUTION_360P)
	}

	return resolutions
}

// getResolutionPath returns the path component based on resolution
func (s *Service) getResolutionPath(resolution pb.VideoResolution) string {
	switch resolution {
	case pb.VideoResolution_VIDEO_RESOLUTION_240P:
		return "240p"
	case pb.VideoResolution_VIDEO_RESOLUTION_360P:
		return "360p"
	case pb.VideoResolution_VIDEO_RESOLUTION_480P:
		return "480p"
	case pb.VideoResolution_VIDEO_RESOLUTION_720P:
		return "720p"
	case pb.VideoResolution_VIDEO_RESOLUTION_1080P:
		return "1080p"
	case pb.VideoResolution_VIDEO_RESOLUTION_1440P:
		return "1440p"
	case pb.VideoResolution_VIDEO_RESOLUTION_2160P:
		return "2160p"
	default:
		return "original"
	}
}