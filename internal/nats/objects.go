package natsclient

import (
	"context"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/b64util"
	"github.com/nats-io/nats.go"
)

func (c *Client) ListObjectBuckets(ctx context.Context) ([]domain.ObjectBucketInfo, error) {
	ch := c.js.ObjectStoreNames()
	var names []string
	for name := range ch {
		names = append(names, name)
	}
	out := make([]domain.ObjectBucketInfo, 0, len(names))
	for _, name := range names {
		os, err := c.js.ObjectStore(name)
		if err != nil {
			return nil, err
		}
		status, err := os.Status()
		if err != nil {
			return nil, err
		}
		out = append(out, domain.ObjectBucketInfo{
			Bucket:      status.Bucket(),
			Description: status.Description(),
			Size:        status.Size(),
		})
	}
	return out, nil
}

func (c *Client) CreateObjectBucket(ctx context.Context, cfg *nats.ObjectStoreConfig) (*domain.ObjectBucketInfo, error) {
	os, err := c.js.CreateObjectStore(cfg)
	if err != nil {
		return nil, err
	}
	status, err := os.Status()
	if err != nil {
		return nil, err
	}
	return &domain.ObjectBucketInfo{
		Bucket:      status.Bucket(),
		Description: status.Description(),
		Size:        status.Size(),
	}, nil
}

func (c *Client) GetObjectBucket(ctx context.Context, bucket string) (*domain.ObjectBucketInfo, error) {
	os, err := c.js.ObjectStore(bucket)
	if err != nil {
		return nil, err
	}
	status, err := os.Status()
	if err != nil {
		return nil, err
	}
	return &domain.ObjectBucketInfo{
		Bucket:      status.Bucket(),
		Description: status.Description(),
		Size:        status.Size(),
	}, nil
}

func (c *Client) DeleteObjectBucket(ctx context.Context, bucket string) error {
	return c.js.DeleteObjectStore(bucket)
}

func (c *Client) ListObjects(ctx context.Context, bucket string, offset, limit int) ([]string, int, error) {
	os, err := c.js.ObjectStore(bucket)
	if err != nil {
		return nil, 0, err
	}
	infos, err := os.List()
	if err != nil {
		return nil, 0, err
	}
	names := make([]string, 0, len(infos))
	for _, info := range infos {
		names = append(names, info.Name)
	}
	page, total := sliceStrings(names, offset, limit)
	return page, total, nil
}

func (c *Client) GetObject(ctx context.Context, bucket, name string) (*domain.ObjectInfo, error) {
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
	defer func() { _ = result.Close() }()

	data, err := readBodyPooled(result)
	if err != nil {
		return nil, err
	}

	return &domain.ObjectInfo{
		Bucket:   bucket,
		Name:     name,
		Size:     info.Size,
		Data:     b64util.EncodeToString(data),
		Modified: info.ModTime,
	}, nil
}

func (c *Client) PutObject(ctx context.Context, bucket, name string, data []byte) (*domain.ObjectInfo, error) {
	os, err := c.js.ObjectStore(bucket)
	if err != nil {
		return nil, err
	}
	info, err := os.PutBytes(name, data)
	if err != nil {
		return nil, err
	}
	return &domain.ObjectInfo{
		Bucket:   bucket,
		Name:     name,
		Size:     info.Size,
		Data:     b64util.EncodeToString(data),
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
