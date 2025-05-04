package cloud

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Storage implements the BlobStorage interface using AWS S3 or compatible services
type S3Storage struct {
	client    *s3.Client
	bucketName string
	region     string
	endpoint   string
}

// S3Config holds the configuration for S3Storage
type S3Config struct {
	BucketName string
	Region     string
	Endpoint   string // Optional for custom S3-compatible services
	AccessKey  string
	SecretKey  string
}

// NewS3Storage creates a new S3Storage instance
func NewS3Storage(ctx context.Context, cfg S3Config) (*S3Storage, error) {
	// Create a custom credentials provider
	provider := credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")

	// Create AWS configuration
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if cfg.Endpoint != "" {
			return aws.Endpoint{
				URL:               cfg.Endpoint,
				HostnameImmutable: true,
				SigningRegion:     cfg.Region,
			}, nil
		}
		// Fallback to default endpoint resolution
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	// Load AWS configuration
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(provider),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(awsCfg)

	// Ensure bucket exists
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(cfg.BucketName),
	})
	if err != nil {
		// If bucket doesn't exist, create it
		_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(cfg.BucketName),
			CreateBucketConfiguration: &types.CreateBucketConfiguration{
				LocationConstraint: types.BucketLocationConstraint(cfg.Region),
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return &S3Storage{
		client:     client,
		bucketName: cfg.BucketName,
		region:     cfg.Region,
		endpoint:   cfg.Endpoint,
	}, nil
}

// GenerateUploadURL generates a presigned URL for uploading a file
func (s *S3Storage) GenerateUploadURL(ctx context.Context, key string, contentType string, expiresIn time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)

	// Create presigned request for PUT operation
	request, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(expiresIn))
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return request.URL, nil
}

// GenerateDownloadURL generates a presigned URL for downloading a file
func (s *S3Storage) GenerateDownloadURL(ctx context.Context, key string, expiresIn time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)

	// Create presigned request for GET operation
	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiresIn))
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return request.URL, nil
}

// DeleteObject deletes a file from S3
func (s *S3Storage) DeleteObject(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// CopyObject copies an object within the same bucket
func (s *S3Storage) CopyObject(ctx context.Context, sourceKey, destinationKey string) error {
	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucketName),
		CopySource: aws.String(fmt.Sprintf("%s/%s", s.bucketName, sourceKey)),
		Key:        aws.String(destinationKey),
	})
	if err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}

	return nil
}

// ListObjects lists objects with a prefix
func (s *S3Storage) ListObjects(ctx context.Context, prefix string, maxKeys int32) ([]string, error) {
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.bucketName),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(maxKeys),
	}

	resp, err := s.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	keys := make([]string, 0, len(resp.Contents))
	for _, item := range resp.Contents {
		keys = append(keys, *item.Key)
	}

	return keys, nil
}

// GetObjectMetadata retrieves metadata for an object
func (s *S3Storage) GetObjectMetadata(ctx context.Context, key string) (map[string]string, error) {
	resp, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	metadata := make(map[string]string)
	for k, v := range resp.Metadata {
		metadata[k] = v
	}

	metadata["ContentType"] = *resp.ContentType
	metadata["ContentLength"] = fmt.Sprintf("%d", resp.ContentLength)

	return metadata, nil
}

// GetObjectURL returns a direct URL to an object in S3
// Note: This is not a pre-signed URL and will only work for public objects
func (s *S3Storage) GetObjectURL(key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucketName, s.region, key)
}