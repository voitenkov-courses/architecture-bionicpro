package cdn

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/voitenkov-courses/architecture-bionicpro/reports-service/internal/config"
)

type Cdn struct {
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	BaseURL     string
	Bucket      string
	mc          *minio.Client
}

func New(cfg *config.Config) *Cdn {
	return &Cdn{
		S3Endpoint:  cfg.Cdn.S3Endpoint,
		S3AccessKey: cfg.Cdn.S3AccessKey,
		S3SecretKey: cfg.Cdn.S3SecretKey,
		BaseURL:     cfg.Cdn.BaseURL,
		Bucket:      cfg.Cdn.Bucket,
	}
}

func (c *Cdn) InitClientAndBucket(ctx context.Context) error {
	var err error

	c.mc, err = minio.New(c.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(c.S3AccessKey, c.S3SecretKey, ""),
		Secure: false,
	})
	if err != nil {
		return err
	}

	c.EnsureBucket(ctx)

	return nil
}

func (c *Cdn) EnsureBucket(ctx context.Context) error {
	err := c.mc.MakeBucket(ctx, c.Bucket, minio.MakeBucketOptions{})
	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := c.mc.BucketExists(ctx, c.Bucket)
		if errBucketExists != nil || !exists {
			return err
		}
	}

	policy := fmt.Sprintf(`{"Version": "2012-10-17","Statement": [{"Action": ["s3:GetObject"],"Effect": "Allow","Principal": {"AWS": ["*"]},"Resource": ["arn:aws:s3:::%s/*"]}]}`, c.Bucket)

	err = c.mc.SetBucketPolicy(ctx, c.Bucket, policy)
	if err != nil {
		return err
	}

	return nil
}

func (c *Cdn) BucketExists(ctx context.Context) (bool, error) {
	exists, err := c.mc.BucketExists(ctx, c.Bucket)
	if err != nil || !exists {
		return false, err
	}

	return true, nil
}

func (c *Cdn) ReportsExists(ctx context.Context, userID uint32) (bool, error) {
	_, err := c.mc.StatObject(ctx, c.Bucket, getObjectKey(userID), minio.StatObjectOptions{})
	if err != nil {
		return false, err
	}

	return true, nil
}

func (c *Cdn) GetReportsByUserID(ctx context.Context, userID uint32) (*[]byte, error) {
	reader, err := c.mc.GetObject(ctx, c.Bucket, getObjectKey(userID), minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var data []byte

	_, err = reader.Read(data)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return &data, nil
}

func (c *Cdn) PutReport(ctx context.Context, userID uint32, data []byte) error {
	_, err := c.mc.PutObject(context.Background(), c.Bucket, getObjectKey(userID), bytes.NewBuffer(data), int64(len(data)), minio.PutObjectOptions{ContentType: "application/json"})
	if err != nil {
		return err
	}

	return nil
}

func (c *Cdn) GetCdnURL(userID uint32) string {
	return c.BaseURL + "/" + c.Bucket + "/" + strconv.Itoa(int(userID)) + "/report.json"
}

func getObjectKey(userID uint32) string {
	return strconv.Itoa(int(userID)) + "/report.json"
}
