package transcode

import (
	"context"
	"fmt"
	"sync"
	"time"
	
	"github.com/google/uuid"
	
	"videostreaming/internal/service/video"
	pb "videostreaming/proto/video"
)

// Storage defines interface for persisting transcoding job information
type Storage interface {
	SaveTranscodingJob(ctx context.Context, job *TranscodingJob) error
	GetTranscodingJobs(ctx context.Context, videoID string) ([]*TranscodingJob, error)
	UpdateTranscodingJob(ctx context.Context, job *TranscodingJob) error
}

// FFmpegClient defines interface for interacting with FFmpeg transcoding
type FFmpegClient interface {
	TranscodeVideo(ctx context.Context, inputPath string, outputPath string, options TranscodeOptions) error
	GetMediaInfo(ctx context.Context, filePath string) (*MediaInfo, error)
}

// BlobStorage defines interface for accessing video files
type BlobStorage interface {
	GenerateDownloadURL(ctx context.Context, key string, expiresIn time.Duration) (string, error)
}

// NotificationService defines interface for sending notifications about transcoding status
type NotificationService interface {
	NotifyTranscodingComplete(ctx context.Context, videoID string, status pb.TranscodingStatus) error
	NotifyTranscodingProgress(ctx context.Context, videoID string, progress float32) error
}

// TranscodingJob represents a video transcoding job
type TranscodingJob struct {
	ID          string
	VideoID     string
	InputPath   string
	OutputPaths map[pb.VideoResolution]string
	Status      pb.TranscodingStatus
	Progress    float32
	Resolution  pb.VideoResolution
	StartedAt   time.Time
	FinishedAt  *time.Time
	Error       string
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
	storage        Storage
	ffmpeg         FFmpegClient
	blobStorage    BlobStorage
	notification   NotificationService
	
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
func NewService(storage Storage, ffmpeg FFmpegClient, blobStorage BlobStorage, notification NotificationService) *Service {
	return &Service{
		storage:        storage,
		ffmpeg:         ffmpeg,
		blobStorage:    blobStorage,
		notification:   notification,
		outputKeyPrefix: "transcoded/",
		availableFormats: []string{"hls", "mp4"},
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

// StartTranscoding begins transcoding a video
func (s *Service) StartTranscoding(ctx context.Context, videoID string, inputPath string) error {
	// Get media info to determine appropriate resolutions
	mediaInfo, err := s.ffmpeg.GetMediaInfo(ctx, inputPath)
	if err != nil {
		return fmt.Errorf("failed to get media info: %w", err)
	}
	
	// Determine resolutions to transcode to based on original video resolution
	resolutions := s.determineTargetResolutions(mediaInfo.Width, mediaInfo.Height)
	
	// Create and save transcoding jobs for each resolution
	jobs := make([]*TranscodingJob, 0, len(resolutions))
	now := time.Now()
	
	for _, resolution := range resolutions {
		outputPath := fmt.Sprintf("%s%s/%s", s.outputKeyPrefix, videoID, s.getResolutionPath(resolution))
		
		job := &TranscodingJob{
			ID:         uuid.New().String(),
			VideoID:    videoID,
			InputPath:  inputPath,
			OutputPaths: map[pb.VideoResolution]string{
				resolution: outputPath,
			},
			Status:     pb.TranscodingStatus_TRANSCODING_STATUS_QUEUED,
			Resolution: resolution,
			StartedAt:  now,
			Progress:   0,
		}
		
		if err := s.storage.SaveTranscodingJob(ctx, job); err != nil {
			return fmt.Errorf("failed to save transcoding job: %w", err)
		}
		
		jobs = append(jobs, job)
	}
	
	// Store job state
	s.jobsLock.Lock()
	s.jobs[videoID] = &jobState{
		videoID:   videoID,
		jobs:      jobs,
		progress:  0,
		status:    pb.TranscodingStatus_TRANSCODING_STATUS_QUEUED,
		updatedAt: now,
	}
	s.jobsLock.Unlock()
	
	// Start transcoding in goroutine
	go s.processTranscodingJobs(context.Background(), videoID, jobs)
	
	return nil
}

// GetTranscodingStatus retrieves the status of a video's transcoding
func (s *Service) GetTranscodingStatus(ctx context.Context, videoID string) (*video.TranscodingStatus, error) {
	// Check in-memory cache first
	s.jobsLock.RLock()
	jobState, exists := s.jobs[videoID]
	s.jobsLock.RUnlock()
	
	var jobs []*TranscodingJob
	var status pb.TranscodingStatus
	var progress float32
	
	if exists {
		jobs = jobState.jobs
		status = jobState.status
		progress = jobState.progress
	} else {
		// Fetch from storage
		var err error
		jobs, err = s.storage.GetTranscodingJobs(ctx, videoID)
		if err != nil {
			return nil, fmt.Errorf("failed to get transcoding jobs: %w", err)
		}
		
		if len(jobs) == 0 {
			return nil, fmt.Errorf("no transcoding jobs found for video: %s", videoID)
		}
		
		// Calculate overall status and progress
		status = s.calculateOverallStatus(jobs)
		progress = s.calculateOverallProgress(jobs)
	}
	
	// Convert to result format
	result := &video.TranscodingStatus{
		VideoID:         videoID,
		Status:          status,
		OverallProgress: progress,
	}
	
	jobsResult := make([]*video.TranscodingJob, 0, len(jobs))
	for _, job := range jobs {
		jobsResult = append(jobsResult, &video.TranscodingJob{
			JobID:        job.ID,
			Resolution:   job.Resolution,
			Status:       job.Status,
			Progress:     job.Progress,
			ErrorMessage: job.Error,
		})
	}
	
	result.Jobs = jobsResult
	
	return result, nil
}

// processTranscodingJobs handles the transcoding workflow
func (s *Service) processTranscodingJobs(ctx context.Context, videoID string, jobs []*TranscodingJob) {
	// Update status to processing
	s.updateJobStatus(ctx, videoID, pb.TranscodingStatus_TRANSCODING_STATUS_PROCESSING, 0)
	
	// Process each job sequentially
	// In a real system, you might want to distribute these jobs to worker nodes
	for _, job := range jobs {
		s.processJob(ctx, job)
	}
	
	// Check final status
	s.jobsLock.RLock()
	jobState, exists := s.jobs[videoID]
	s.jobsLock.RUnlock()
	
	if exists {
		finalStatus := s.calculateOverallStatus(jobState.jobs)
		s.updateJobStatus(ctx, videoID, finalStatus, 100)
		
		// Notify about completion
		s.notification.NotifyTranscodingComplete(ctx, videoID, finalStatus)
	}
}

// processJob handles a single transcoding job
func (s *Service) processJob(ctx context.Context, job *TranscodingJob) {
	// Update job status to processing
	job.Status = pb.TranscodingStatus_TRANSCODING_STATUS_PROCESSING
	job.Progress = 0
	
	if err := s.storage.UpdateTranscodingJob(ctx, job); err != nil {
		fmt.Printf("Failed to update job status: %v\n", err)
	}
	
	// Create transcoding options
	options := TranscodeOptions{
		Resolution:  job.Resolution,
		VideoBitrate: s.bitrates[job.Resolution],
		AudioBitrate: s.audioBitrate,
		Format:       "hls", // Use HLS for adaptive streaming
		Codec:        s.codec,
		FrameRate:    30,
	}
	
	// Get output path for this resolution
	outputPath, ok := job.OutputPaths[job.Resolution]
	if !ok {
		job.Status = pb.TranscodingStatus_TRANSCODING_STATUS_FAILED
		job.Error = "missing output path"
		s.updateJobStatus(ctx, job.VideoID, job.Status, job.Progress)
		return
	}
	
	// Start transcoding
	err := s.ffmpeg.TranscodeVideo(ctx, job.InputPath, outputPath, options)
	
	now := time.Now()
	if err != nil {
		job.Status = pb.TranscodingStatus_TRANSCODING_STATUS_FAILED
		job.Error = err.Error()
		job.Progress = 0
	} else {
		job.Status = pb.TranscodingStatus_TRANSCODING_STATUS_COMPLETED
		job.Progress = 100
	}
	
	job.FinishedAt = &now
	
	// Update job in storage
	if err := s.storage.UpdateTranscodingJob(ctx, job); err != nil {
		fmt.Printf("Failed to update job: %v\n", err)
	}
	
	// Update in-memory state
	s.jobsLock.Lock()
	if jobState, ok := s.jobs[job.VideoID]; ok {
		for i, j := range jobState.jobs {
			if j.ID == job.ID {
				jobState.jobs[i] = job
				break
			}
		}
		jobState.status = s.calculateOverallStatus(jobState.jobs)
		jobState.progress = s.calculateOverallProgress(jobState.jobs)
		jobState.updatedAt = now
	}
	s.jobsLock.Unlock()
}

// updateJobStatus updates the status and progress of a transcoding job
func (s *Service) updateJobStatus(ctx context.Context, videoID string, status pb.TranscodingStatus, progress float32) {
	s.jobsLock.Lock()
	if jobState, ok := s.jobs[videoID]; ok {
		jobState.status = status
		jobState.progress = progress
		jobState.updatedAt = time.Now()
	}
	s.jobsLock.Unlock()
	
	// Send notification about progress
	s.notification.NotifyTranscodingProgress(ctx, videoID, progress)
}

// calculateOverallStatus determines the overall status of transcoding jobs
func (s *Service) calculateOverallStatus(jobs []*TranscodingJob) pb.TranscodingStatus {
	if len(jobs) == 0 {
		return pb.TranscodingStatus_TRANSCODING_STATUS_UNSPECIFIED
	}
	
	hasQueued := false
	hasProcessing := false
	hasFailures := false
	
	for _, job := range jobs {
		switch job.Status {
		case pb.TranscodingStatus_TRANSCODING_STATUS_FAILED:
			hasFailures = true
		case pb.TranscodingStatus_TRANSCODING_STATUS_PROCESSING:
			hasProcessing = true
		case pb.TranscodingStatus_TRANSCODING_STATUS_QUEUED:
			hasQueued = true
		}
	}
	
	// Determine overall status
	if hasProcessing || hasQueued {
		return pb.TranscodingStatus_TRANSCODING_STATUS_PROCESSING
	} else if hasFailures {
		// If there are failures but some jobs completed, consider it partially completed
		hasCompleted := false
		for _, job := range jobs {
			if job.Status == pb.TranscodingStatus_TRANSCODING_STATUS_COMPLETED {
				hasCompleted = true
				break
			}
		}
		if hasCompleted {
			return pb.TranscodingStatus_TRANSCODING_STATUS_COMPLETED
		}
		return pb.TranscodingStatus_TRANSCODING_STATUS_FAILED
	}
	
	return pb.TranscodingStatus_TRANSCODING_STATUS_COMPLETED
}

// calculateOverallProgress calculates the overall progress of transcoding jobs
func (s *Service) calculateOverallProgress(jobs []*TranscodingJob) float32 {
	if len(jobs) == 0 {
		return 0
	}
	
	var totalProgress float32
	for _, job := range jobs {
		totalProgress += job.Progress
	}
	
	return totalProgress / float32(len(jobs))
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