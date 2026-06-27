package natsclient

import (
	"context"
	"encoding/base64"
	"io"
	"time"

	"github.com/nats-io/nats.go"
)

type ObjectBucketInfo struct {
	Bucket      string `json:"bucket"`
	Description string `json:"description"`
	Size        uint64 `json:"size"`
}

type ObjectInfo struct {
	Bucket   string    `json:"bucket"`
	Name     string    `json:"name"`
	Size     uint64    `json:"size"`
	Data     string    `json:"data"`
	Modified time.Time `json:"modified"`
}

func (c *Client) ListObjectBuckets(ctx context.Context) ([]ObjectBucketInfo, error) {
	ch := c.js.ObjectStoreNames()
	var names []string
	for name := range ch {
		names = append(names, name)
	}
	out := make([]ObjectBucketInfo, 0, len(names))
	for _, name := range names {
		os, err := c.js.ObjectStore(name)
		if err != nil {
			return nil, err
		}
		status, err := os.Status()
		if err != nil {
			return nil, err
		}
		out = append(out, ObjectBucketInfo{
			Bucket:      status.Bucket(),
			Description: status.Description(),
			Size:        status.Size(),
		})
	}
	return out, nil
}

func (c *Client) CreateObjectBucket(ctx context.Context, cfg *nats.ObjectStoreConfig) (*ObjectBucketInfo, error) {
	os, err := c.js.CreateObjectStore(cfg)
	if err != nil {
		return nil, err
	}
	status, err := os.Status()
	if err != nil {
		return nil, err
	}
	return &ObjectBucketInfo{
		Bucket:      status.Bucket(),
		Description: status.Description(),
		Size:        status.Size(),
	}, nil
}

func (c *Client) GetObjectBucket(ctx context.Context, bucket string) (*ObjectBucketInfo, error) {
	os, err := c.js.ObjectStore(bucket)
	if err != nil {
		return nil, err
	}
	status, err := os.Status()
	if err != nil {
		return nil, err
	}
	return &ObjectBucketInfo{
		Bucket:      status.Bucket(),
		Description: status.Description(),
		Size:        status.Size(),
	}, nil
}

func (c *Client) DeleteObjectBucket(ctx context.Context, bucket string) error {
	return c.js.DeleteObjectStore(bucket)
}

func (c *Client) ListObjects(ctx context.Context, bucket string) ([]string, error) {
	os, err := c.js.ObjectStore(bucket)
	if err != nil {
		return nil, err
	}
	infos, err := os.List()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(infos))
	for _, info := range infos {
		names = append(names, info.Name)
	}
	return names, nil
}

func (c *Client) GetObject(ctx context.Context, bucket, name string) (*ObjectInfo, error) {
	os, err := c.js.ObjectStore(bucket)
	if err != nil {
		return nil, err
	}
	info, err := os.GetInfo(name)
	if err != nil {
		return nil, err
	}
	result, err := os.Get(name)
	if err != nil {
		return nil, err
	}
	defer result.Close()

	data, err := io.ReadAll(result)
	if err != nil {
		return nil, err
	}

	return &ObjectInfo{
		Bucket:   bucket,
		Name:     name,
		Size:     info.Size,
		Data:     base64.StdEncoding.EncodeToString(data),
		Modified: info.ModTime,
	}, nil
}

func (c *Client) PutObject(ctx context.Context, bucket, name string, data []byte) (*ObjectInfo, error) {
	os, err := c.js.ObjectStore(bucket)
	if err != nil {
		return nil, err
	}
	info, err := os.PutBytes(name, data)
	if err != nil {
		return nil, err
	}
	return &ObjectInfo{
		Bucket:   bucket,
		Name:     name,
		Size:     info.Size,
		Data:     base64.StdEncoding.EncodeToString(data),
		Modified: info.ModTime,
	}, nil
}

func (c *Client) DeleteObject(ctx context.Context, bucket, name string) error {
	os, err := c.js.ObjectStore(bucket)
	if err != nil {
		return err
	}
	return os.Delete(name)
}
