package domain

import "time"

type ObjectBucketInfo struct {
	Bucket      string `json:"bucket"`
	Description string `json:"description"`
	Size        uint64 `json:"size"`
}

type ObjectInfo struct {
	Modified time.Time `json:"modified"`
	Bucket   string    `json:"bucket"`
	Name     string    `json:"name"`
	Data     string    `json:"data"`
	Size     uint64    `json:"size"`
}
