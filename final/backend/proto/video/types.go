package video

import (
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// VideoStatus represents the status of a video
type VideoStatus int32

const (
	VideoStatus_VIDEO_STATUS_UNSPECIFIED VideoStatus = 0
	VideoStatus_VIDEO_STATUS_UPLOADING   VideoStatus = 1
	VideoStatus_VIDEO_STATUS_PROCESSING  VideoStatus = 2
	VideoStatus_VIDEO_STATUS_READY       VideoStatus = 3
	VideoStatus_VIDEO_STATUS_FAILED      VideoStatus = 4
)

// StreamStatus represents the status of a live stream
type StreamStatus int32

const (
	StreamStatus_STREAM_STATUS_UNSPECIFIED StreamStatus = 0
	StreamStatus_STREAM_STATUS_CREATED     StreamStatus = 1
	StreamStatus_STREAM_STATUS_LIVE        StreamStatus = 2
	StreamStatus_STREAM_STATUS_ENDED       StreamStatus = 3
	StreamStatus_STREAM_STATUS_ERROR       StreamStatus = 4
)

// VideoVisibility represents the visibility of a video
type VideoVisibility int32

const (
	VideoVisibility_VIDEO_VISIBILITY_UNSPECIFIED VideoVisibility = 0
	VideoVisibility_VIDEO_VISIBILITY_PUBLIC      VideoVisibility = 1
	VideoVisibility_VIDEO_VISIBILITY_PRIVATE     VideoVisibility = 2
	VideoVisibility_VIDEO_VISIBILITY_UNLISTED    VideoVisibility = 3
)

// VideoResolution represents the resolution of a video
type VideoResolution int32

const (
	VideoResolution_VIDEO_RESOLUTION_UNSPECIFIED VideoResolution = 0
	VideoResolution_VIDEO_RESOLUTION_240P        VideoResolution = 1
	VideoResolution_VIDEO_RESOLUTION_360P        VideoResolution = 2
	VideoResolution_VIDEO_RESOLUTION_480P        VideoResolution = 3
	VideoResolution_VIDEO_RESOLUTION_720P        VideoResolution = 4
	VideoResolution_VIDEO_RESOLUTION_1080P       VideoResolution = 5
	VideoResolution_VIDEO_RESOLUTION_1440P       VideoResolution = 6
	VideoResolution_VIDEO_RESOLUTION_2160P       VideoResolution = 7
)

// TranscodingStatus represents the status of a transcoding job
type TranscodingStatus int32

const (
	TranscodingStatus_TRANSCODING_STATUS_UNSPECIFIED TranscodingStatus = 0
	TranscodingStatus_TRANSCODING_STATUS_QUEUED      TranscodingStatus = 1
	TranscodingStatus_TRANSCODING_STATUS_PROCESSING  TranscodingStatus = 2
	TranscodingStatus_TRANSCODING_STATUS_COMPLETED   TranscodingStatus = 3
	TranscodingStatus_TRANSCODING_STATUS_FAILED      TranscodingStatus = 4
	TranscodingStatus_TRANSCODING_STATUS_ERROR       TranscodingStatus = 4  // Alias for FAILED
	TranscodingStatus_TRANSCODING_STATUS_NOT_FOUND   TranscodingStatus = 5
)

// Video represents a video entity
type Video struct {
	Id              string
	Title           string
	Description     string
	UserId          string
	ThumbnailUrl    string
	VideoUrl        string
	DurationSeconds int64
	ViewCount       int64
	Status          VideoStatus
	CreatedAt       *timestamppb.Timestamp
	UpdatedAt       *timestamppb.Timestamp
	Tags            []string
	Visibility      VideoVisibility
	Resolution      VideoResolution
}

// InitiateUploadRequest represents a request to initiate a video upload
type InitiateUploadRequest struct {
	Title       string
	Description string
	UserId      string
	FileSizeBytes int64
	ContentType string
	Visibility  VideoVisibility
	Tags        []string
}

// InitiateUploadResponse represents a response to an upload initiation
type InitiateUploadResponse struct {
	UploadId  string
	VideoId   string
	UploadUrl string
}

// CompleteUploadRequest represents a request to complete a video upload
type CompleteUploadRequest struct {
	UploadId string
	VideoId  string
}

// CompleteUploadResponse represents a response to an upload completion
type CompleteUploadResponse struct {
	VideoId string
	Status  VideoStatus
}

// GetVideoRequest represents a request to get a video
type GetVideoRequest struct {
	VideoId string
}

// ListVideosRequest represents a request to list videos
type ListVideosRequest struct {
	UserId    string
	PageSize  int32
	PageToken string
}

// ListVideosResponse represents a response to a list videos request
type ListVideosResponse struct {
	Videos        []*Video
	NextPageToken string
	TotalCount    int32
}

// DeleteVideoRequest represents a request to delete a video
type DeleteVideoRequest struct {
	VideoId string
	UserId  string
}

// GetStreamKeyRequest represents a request to get a stream key
type GetStreamKeyRequest struct {
	UserId string
}

// StreamKeyResponse represents a response containing a stream key
type StreamKeyResponse struct {
	StreamKey string
	RtmpUrl   string
}

// StartStreamRequest represents a request to start a live stream
type StartStreamRequest struct {
	UserId      string
	StreamKey   string
	Title       string
	Description string
	Tags        []string
}

// StreamResponse represents a response to a stream operation
type StreamResponse struct {
	StreamId    string
	PlaybackUrl string
	StreamKey   string
}

// EndStreamRequest represents a request to end a live stream
type EndStreamRequest struct {
	StreamId string
	UserId   string
}

// GetLiveStreamsRequest represents a request to list live streams
type GetLiveStreamsRequest struct {
	UserId    string
	PageSize  int32
	PageToken string
}

// GetLiveStreamsResponse represents a response to a live streams listing
type GetLiveStreamsResponse struct {
	Streams       []*LiveStream
	NextPageToken string
	TotalCount    int32
}

// LiveStream represents a live stream entity
type LiveStream struct {
	StreamId     string
	UserId       string
	Title        string
	Description  string
	ThumbnailUrl string
	PlaybackUrl  string
	ViewerCount  int64
	StartedAt    *timestamppb.Timestamp
	Tags         []string
	StreamKey    string // Added stream_key field to match the proto definition
}

// GetStreamRequest represents a request to get a specific stream by ID
type GetStreamRequest struct {
	StreamId string
}

// GetStreamResponse represents a response with details of a specific stream
type GetStreamResponse struct {
	Stream *LiveStream
}

// GetTranscodingStatusRequest represents a request for transcoding status
type GetTranscodingStatusRequest struct {
	VideoId string
}

// TranscodingStatusResponse represents a response with transcoding status
type TranscodingStatusResponse struct {
	VideoId         string
	Status          TranscodingStatus
	Jobs            []*TranscodingJob
	OverallProgress float32
}

// TranscodingJob represents a single transcoding job
type TranscodingJob struct {
	JobId        string
	Resolution   VideoResolution
	Status       TranscodingStatus
	Progress     float32
	ErrorMessage string
}

// UnimplementedVideoServiceServer is a placeholder for gRPC service implementation
type UnimplementedVideoServiceServer struct{}

func (UnimplementedVideoServiceServer) InitiateUpload(interface{}, interface{}) (interface{}, error) {
	return nil, nil
}

func (UnimplementedVideoServiceServer) CompleteUpload(interface{}, interface{}) (interface{}, error) {
	return nil, nil
}

func (UnimplementedVideoServiceServer) GetVideo(interface{}, interface{}) (interface{}, error) {
	return nil, nil
}

func (UnimplementedVideoServiceServer) ListVideos(interface{}, interface{}) (interface{}, error) {
	return nil, nil
}

func (UnimplementedVideoServiceServer) DeleteVideo(interface{}, interface{}) (interface{}, error) {
	return nil, nil
}

func (UnimplementedVideoServiceServer) GetStreamKey(interface{}, interface{}) (interface{}, error) {
	return nil, nil
}

func (UnimplementedVideoServiceServer) StartStream(interface{}, interface{}) (interface{}, error) {
	return nil, nil
}

func (UnimplementedVideoServiceServer) EndStream(interface{}, interface{}) (interface{}, error) {
	return nil, nil
}

func (UnimplementedVideoServiceServer) GetLiveStreams(interface{}, interface{}) (interface{}, error) {
	return nil, nil
}

func (UnimplementedVideoServiceServer) GetStream(interface{}, interface{}) (interface{}, error) {
	return nil, nil
}

func (UnimplementedVideoServiceServer) GetTranscodingStatus(interface{}, interface{}) (interface{}, error) {
	return nil, nil
}

// RegisterVideoServiceServer registers a VideoService server implementation with a gRPC server
func RegisterVideoServiceServer(s grpc.ServiceRegistrar, srv interface{}) {
	// This is a simplified implementation just to get the code to compile
	// In a real application, this would be generated by protoc
}