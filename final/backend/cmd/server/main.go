package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"

	"videostreaming/internal/service/streaming"
	"videostreaming/internal/service/transcode"
	"videostreaming/internal/service/video"
	"videostreaming/internal/storage/filesystem"
	"videostreaming/internal/storage/memory"
	pb "videostreaming/proto/video"
)

// TranscodingServiceAdapter adapts transcode.Service to video.TranscodingService
type TranscodingServiceAdapter struct {
	transcodeService *transcode.Service
}

// StartTranscoding delegates to the underlying transcode service
func (a *TranscodingServiceAdapter) StartTranscoding(ctx context.Context, videoID string, inputPath string) error {
	return a.transcodeService.StartTranscoding(ctx, videoID, inputPath)
}

// GetTranscodingStatus adapts the response from transcode service to video service
func (a *TranscodingServiceAdapter) GetTranscodingStatus(ctx context.Context, videoID string) (*video.TranscodingStatus, error) {
	status, err := a.transcodeService.GetTranscodingStatus(ctx, videoID)
	if err != nil {
		return nil, err
	}
	
	// Convert the proto response to video.TranscodingStatus
	jobs := make([]*video.TranscodingJob, 0, len(status.Jobs))
	for _, job := range status.Jobs {
		jobs = append(jobs, &video.TranscodingJob{
			JobID:        job.JobId,
			Resolution:   job.Resolution,
			Status:       job.Status,
			Progress:     job.Progress,
			ErrorMessage: job.ErrorMessage,
		})
	}
	
	return &video.TranscodingStatus{
		VideoID:         status.VideoId,
		Status:          status.Status,
		Jobs:            jobs,
		OverallProgress: status.OverallProgress,
	}, nil
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Set up storage directories
	mediaDir := getEnv("MEDIA_DIR", "./media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		log.Fatalf("Failed to create media directory: %v", err)
	}

	// Create file storage for videos and thumbnails
	baseURL := getEnv("BASE_URL", "http://localhost:8080")
	fileStorage, err := filesystem.NewFileSystemStorage(mediaDir, baseURL)
	if err != nil {
		log.Fatalf("Failed to create file storage: %v", err)
	}

	// Create in-memory storage for video metadata
	videoStorage := memory.NewVideoStorage()

	// Create mock implementations for development
	ffmpegClient := &mockFFmpegClient{}
	notificationService := &mockNotificationService{}
	
	// Create real MediaMTX streaming engine
	// The MediaMTX server is running on:
	// - RTMP port 1935
	// - HLS port 8888
	// - WebRTC port 8889
	streamingEngine := streaming.NewMediaMTXEngine(
		getEnv("RTMP_URL", "rtmp://localhost:1935/live"),
		getEnv("HLS_URL", "http://localhost:8888/live"),
		getEnv("WEBRTC_URL", "http://localhost:8889/live"),
	)
	
	// Create transcoding service
	transcodingService := transcode.NewService(
		&mockTranscodeStorage{}, 
		ffmpegClient, 
		fileStorage, // Use fileStorage instead of S3Storage 
		notificationService,
	)
	
	// Create adapter for the transcoding service
	transcodeAdapter := &TranscodingServiceAdapter{
		transcodeService: transcodingService,
	}

	// Create video service
	videoService := video.NewService(
		videoStorage,
		fileStorage, // Use fileStorage instead of S3Storage
		transcodeAdapter, // Use the adapter instead of the raw transcoding service
		streamingEngine,
	)

	// Start gRPC server
	go startGRPCServer(videoService)

	// Start REST API server
	go startRESTServer(videoService, fileStorage)

	// Wait for termination signal
	waitForSignal()
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

func startRESTServer(videoService *video.Service, fileStorage *filesystem.FileSystemStorage) {
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

	// File upload/download endpoints
	router.Post("/upload", handleFileUpload(fileStorage))
	router.Get("/download/{path:.+}", handleFileDownload(fileStorage))

	// API routes
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
			r.Get("/{streamID}", handleGetStream(videoService)) // Add this line to handle GET request for a specific stream
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

// File handling functions

func handleFileUpload(fs *filesystem.FileSystemStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get the path from the query
		path := r.URL.Query().Get("path")
		if path == "" {
			http.Error(w, "Path is required", http.StatusBadRequest)
			return
		}

		// Parse the multipart form, 32MB max
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
			return
		}

		// Get the file
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get file: %v", err), http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Read the file
		buffer := make([]byte, header.Size)
		if _, err := file.Read(buffer); err != nil {
			http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusInternalServerError)
			return
		}

		// Save the file
		if err := fs.SaveFile(path, buffer); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save file: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}
}

func handleFileDownload(fs *filesystem.FileSystemStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pathParam := chi.URLParam(r, "path")
		if pathParam == "" {
			http.Error(w, "Path is required", http.StatusBadRequest)
			return
		}

		// Clean the path to prevent directory traversal
		path := filepath.Clean(pathParam)

		// Get the file data
		data, err := fs.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "File not found", http.StatusNotFound)
			} else {
				http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusInternalServerError)
			}
			return
		}

		// Set content type based on file extension
		contentType := http.DetectContentType(data)
		if ext := filepath.Ext(path); ext != "" {
			switch ext {
			case ".mp4":
				contentType = "video/mp4"
			case ".m3u8":
				contentType = "application/vnd.apple.mpegurl"
			case ".ts":
				contentType = "video/mp2t"
			case ".jpg", ".jpeg":
				contentType = "image/jpeg"
			case ".png":
				contentType = "image/png"
			}
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}
}

// Mock implementations for development purposes

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
			// Get query parameters
		userID := r.URL.Query().Get("user_id")
		pageSizeStr := r.URL.Query().Get("page_size")
		pageToken := r.URL.Query().Get("page_token")
		
		pageSize := int32(20) // Default page size
		if pageSizeStr != "" {
			if size, err := strconv.Atoi(pageSizeStr); err == nil && size > 0 {
				pageSize = int32(size)
			}
		}
		
		// Call the service to list streams
		response, err := svc.GetLiveStreams(r.Context(), &pb.GetLiveStreamsRequest{
			UserId:    userID,
			PageSize:  pageSize,
			PageToken: pageToken,
		})
		
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to list streams: %v", err), http.StatusInternalServerError)
			return
		}
		
		// Convert the streams to a format suitable for JSON
		streams := make([]map[string]interface{}, 0, len(response.Streams))
		for _, stream := range response.Streams {
			streams = append(streams, map[string]interface{}{
				"stream_id":     stream.StreamId,
				"user_id":       stream.UserId,
				"title":         stream.Title,
				"description":   stream.Description,
				"thumbnail_url": stream.ThumbnailUrl,
				"playback_url":  stream.PlaybackUrl,
				"viewer_count":  stream.ViewerCount,
				"started_at":    stream.StartedAt.AsTime(),
				"tags":          stream.Tags,
			})
		}
		
		// Send response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"streams":        streams,
			"next_page_token": response.NextPageToken,
			"total_count":    response.TotalCount,
		})
	}
}

func handleGetStreamKey(svc *video.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse request body
		var requestData struct {
			UserID string `json:"user_id"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		
		// Call the service to get or generate stream key
		response, err := svc.GetStreamKey(r.Context(), &pb.GetStreamKeyRequest{
			UserId: requestData.UserID,
		})
		
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get stream key: %v", err), http.StatusInternalServerError)
			return
		}
		
		// Send response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"stream_key": response.StreamKey,
			"rtmp_url":   response.RtmpUrl,
		})
	}
}

func handleStartStream(svc *video.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse request body
		var requestData struct {
			UserID      string   `json:"user_id"`
			StreamKey   string   `json:"stream_key"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Tags        []string `json:"tags"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		
		// Call the service to start the stream
		response, err := svc.StartStream(r.Context(), &pb.StartStreamRequest{
			UserId:      requestData.UserID,
			StreamKey:   requestData.StreamKey,
			Title:       requestData.Title,
			Description: requestData.Description,
			Tags:        requestData.Tags,
		})
		
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to start stream: %v", err), http.StatusInternalServerError)
			return
		}
		
		// Send response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"stream_id":    response.StreamId,
			"playback_url": response.PlaybackUrl,
			"stream_key":   response.StreamKey,
		})
	}
}

func handleEndStream(svc *video.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get stream ID from URL params
		streamID := chi.URLParam(r, "streamID")
		
		// Parse request body
		var requestData struct {
			UserID string `json:"user_id"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		
		// Call the service to end the stream
		_, err := svc.EndStream(r.Context(), &pb.EndStreamRequest{
			StreamId: streamID,
			UserId:   requestData.UserID,
		})
		
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to end stream: %v", err), http.StatusInternalServerError)
			return
		}
		
		// Send success response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{
			"success": true,
		})
	}
}

func handleGetStream(svc *video.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get stream ID from URL params
		streamID := chi.URLParam(r, "streamID")
		
		// Call the service to get stream details
		response, err := svc.GetStream(r.Context(), &pb.GetStreamRequest{
			StreamId: streamID,
		})
		
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get stream: %v", err), http.StatusInternalServerError)
			return
		}
		
		// Send response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stream_id":     response.Stream.StreamId,
			"user_id":       response.Stream.UserId,
			"title":         response.Stream.Title,
			"description":   response.Stream.Description,
			"thumbnail_url": response.Stream.ThumbnailUrl,
			"playback_url":  response.Stream.PlaybackUrl,
			"viewer_count":  response.Stream.ViewerCount,
			"started_at":    response.Stream.StartedAt.AsTime(),
			"tags":          response.Stream.Tags,
		})
	}
}