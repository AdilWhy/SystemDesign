package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"

	"videostreaming/internal/service/transcode"
	"videostreaming/internal/service/video"
	"videostreaming/internal/storage/memory"
	pb "videostreaming/proto/video"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Initialize S3 storage with a mock implementation for development
	s3Storage := &mockS3Storage{}

	// Create storage implementations
	// Using in-memory storage for development
	videoStorage := memory.NewVideoStorage()

	// Create mock implementations for development
	// In production, these would be real implementations
	ffmpegClient := &mockFFmpegClient{}
	notificationService := &mockNotificationService{}
	streamingEngine := &mockStreamingEngine{}
	
	// Create transcoding service
	transcodingService := transcode.NewService(
		&mockTranscodeStorage{}, 
		ffmpegClient, 
		s3Storage, 
		notificationService,
	)

	// Create video service
	videoService := video.NewService(
		videoStorage,
		s3Storage,
		transcodingService,
		streamingEngine,
	)

	// Start gRPC server
	go startGRPCServer(videoService)

	// Start REST API server
	go startRESTServer(videoService)

	// Wait for termination signal
	waitForSignal()
}

// mockS3Storage implements a simple mock of the S3 storage for testing
type mockS3Storage struct{}

func (m *mockS3Storage) GenerateUploadURL(ctx context.Context, key string, contentType string, expiresIn time.Duration) (string, error) {
	return fmt.Sprintf("https://mock-upload-url.example.com/%s", key), nil
}

func (m *mockS3Storage) GenerateDownloadURL(ctx context.Context, key string, expiresIn time.Duration) (string, error) {
	return fmt.Sprintf("https://mock-download-url.example.com/%s", key), nil
}

func (m *mockS3Storage) DeleteObject(ctx context.Context, key string) error {
	return nil
}

func startGRPCServer(videoService *video.Service) {
	port := getEnv("GRPC_PORT", "50051")
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterVideoServiceServer(grpcServer, videoService)

	log.Printf("Starting gRPC server on port %s", port)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to start gRPC server: %v", err)
	}
}

func startRESTServer(videoService *video.Service) {
	router := chi.NewRouter()

	// Middleware
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	// Define your REST API routes here
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Example video routes
	router.Route("/api/v1", func(r chi.Router) {
		r.Route("/videos", func(r chi.Router) {
			r.Get("/", handleListVideos(videoService))
			r.Post("/", handleInitiateUpload(videoService))
			r.Get("/{videoID}", handleGetVideo(videoService))
			r.Delete("/{videoID}", handleDeleteVideo(videoService))
			r.Post("/{videoID}/complete", handleCompleteUpload(videoService))
		})

		r.Route("/streams", func(r chi.Router) {
			r.Get("/", handleListStreams(videoService))
			r.Post("/key", handleGetStreamKey(videoService))
			r.Post("/", handleStartStream(videoService))
			r.Delete("/{streamID}", handleEndStream(videoService))
		})
	})

	port := getEnv("HTTP_PORT", "8080")
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: router,
	}

	log.Printf("Starting REST server on port %s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start REST server: %v", err)
	}
}

func waitForSignal() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	log.Printf("Received signal: %s, shutting down...", sig)
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// Mock implementations for development purposes
// In production, these would be replaced with real implementations

type mockFFmpegClient struct{}

func (m *mockFFmpegClient) TranscodeVideo(ctx context.Context, inputPath string, outputPath string, options transcode.TranscodeOptions) error {
	log.Printf("Mocking transcoding: %s to %s with resolution %v", inputPath, outputPath, options.Resolution)
	// Simulate transcoding delay
	time.Sleep(2 * time.Second)
	return nil
}

func (m *mockFFmpegClient) GetMediaInfo(ctx context.Context, filePath string) (*transcode.MediaInfo, error) {
	// Return mock media info
	return &transcode.MediaInfo{
		Duration:  120.5,
		Width:     1920,
		Height:    1080,
		Bitrate:   5000000,
		Codec:     "h264",
		FrameRate: 30.0,
	}, nil
}

type mockTranscodeStorage struct{}

func (m *mockTranscodeStorage) SaveTranscodingJob(ctx context.Context, job *transcode.TranscodingJob) error {
	log.Printf("Saving transcoding job: %s for video: %s", job.ID, job.VideoID)
	return nil
}

func (m *mockTranscodeStorage) GetTranscodingJobs(ctx context.Context, videoID string) ([]*transcode.TranscodingJob, error) {
	// Return mock jobs
	jobs := []*transcode.TranscodingJob{
		{
			ID:        "job-1",
			VideoID:   videoID,
			Status:    pb.TranscodingStatus_TRANSCODING_STATUS_COMPLETED,
			Progress:  100.0,
			Resolution: pb.VideoResolution_VIDEO_RESOLUTION_720P,
		},
	}
	return jobs, nil
}

func (m *mockTranscodeStorage) UpdateTranscodingJob(ctx context.Context, job *transcode.TranscodingJob) error {
	log.Printf("Updating transcoding job: %s, progress: %.2f%%", job.ID, job.Progress)
	return nil
}

type mockNotificationService struct{}

func (m *mockNotificationService) NotifyTranscodingComplete(ctx context.Context, videoID string, status pb.TranscodingStatus) error {
	log.Printf("Transcoding complete for video: %s with status: %v", videoID, status)
	return nil
}

func (m *mockNotificationService) NotifyTranscodingProgress(ctx context.Context, videoID string, progress float32) error {
	log.Printf("Transcoding progress for video: %s is %.2f%%", videoID, progress)
	return nil
}

type mockStreamingEngine struct{}

func (m *mockStreamingEngine) GenerateStreamKey(ctx context.Context, userID string) (string, error) {
	return fmt.Sprintf("stream-key-%s", userID), nil
}

func (m *mockStreamingEngine) GetRTMPURL() string {
	return "rtmp://streaming.example.com/live"
}

func (m *mockStreamingEngine) GetStreamPlaybackURL(streamID string) string {
	return fmt.Sprintf("https://streaming.example.com/hls/%s.m3u8", streamID)
}

// HTTP handler implementations

func handleListVideos(svc *video.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Implementation for listing videos
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"videos": []}`))
	}
}

func handleGetVideo(svc *video.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Implementation for getting video details
		videoID := chi.URLParam(r, "videoID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"id": "%s", "title": "Sample Video"}`, videoID)))
	}
}

func handleInitiateUpload(svc *video.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Implementation for initiating video upload
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"upload_id": "upload-123", "upload_url": "https://example.com/upload"}`))
	}
}

func handleCompleteUpload(svc *video.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Implementation for completing video upload
		videoID := chi.URLParam(r, "videoID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"video_id": "%s", "status": "processing"}`, videoID)))
	}
}

func handleDeleteVideo(svc *video.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Implementation for deleting video
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}
}

func handleListStreams(svc *video.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Implementation for listing live streams
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"streams": []}`))
	}
}

func handleGetStreamKey(svc *video.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Implementation for getting stream key
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"stream_key": "sample-key", "rtmp_url": "rtmp://example.com/live"}`))
	}
}

func handleStartStream(svc *video.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Implementation for starting stream
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"stream_id": "stream-123", "playback_url": "https://example.com/stream.m3u8"}`))
	}
}

func handleEndStream(svc *video.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Implementation for ending stream
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}
}