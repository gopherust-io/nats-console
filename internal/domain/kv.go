package domain

import "time"

type KVBucketInfo struct {
	Bucket  string `json:"bucket"`
	Values  uint64 `json:"values"`
	History int64  `json:"history"`
}

type KVEntry struct {
	Created  time.Time `json:"created"`
	Bucket   string    `json:"bucket"`
	Key      string    `json:"key"`
	Value    string    `json:"value"`
	Revision uint64    `json:"revision"`
}
