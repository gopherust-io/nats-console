package domain

import "time"

type PprofConfig struct {
	Profiles           []string `json:"profiles"`
	MaxCPUSecs         int      `json:"maxCpuSeconds"`
	ContinuousInterval int      `json:"continuousIntervalSecs"`
	ContinuousCPUSlice int      `json:"continuousCpuSliceSecs"`
	Enabled            bool     `json:"enabled"`
	AuthRequired       bool     `json:"authRequired"`
	ContinuousEnabled  bool     `json:"continuousEnabled"`
}

type PprofContinuousSnapshot struct {
	FetchedAt      time.Time                      `json:"fetchedAt"`
	Profiles       map[string]PprofProfileSummary `json:"profiles"`
	RuntimeHistory []PprofRuntimeStats            `json:"runtimeHistory"`
	IntervalSecs   int                            `json:"intervalSecs"`
	CPUSliceSecs   int                            `json:"cpuSliceSecs"`
}

type PprofRuntimeStats struct {
	FetchedAt  time.Time        `json:"fetchedAt"`
	Goroutines int              `json:"goroutines"`
	Memory     PprofMemoryStats `json:"memory"`
}

type PprofMemoryStats struct {
	Alloc       uint64 `json:"alloc"`
	TotalAlloc  uint64 `json:"totalAlloc"`
	Sys         uint64 `json:"sys"`
	HeapAlloc   uint64 `json:"heapAlloc"`
	HeapInuse   uint64 `json:"heapInuse"`
	HeapObjects uint64 `json:"heapObjects"`
	NumGC       uint32 `json:"numGc"`
}

type PprofProfileSummary struct {
	FetchedAt    time.Time           `json:"fetchedAt"`
	ProfileType  string              `json:"profileType"`
	Entries      []PprofProfileEntry `json:"entries"`
	TotalSamples int64               `json:"totalSamples,omitempty"`
	DurationSecs int                 `json:"durationSecs,omitempty"`
}

type PprofProfileEntry struct {
	Name        string  `json:"name"`
	Flat        int64   `json:"flat"`
	FlatPercent float64 `json:"flatPercent"`
	Cum         int64   `json:"cum"`
	CumPercent  float64 `json:"cumPercent"`
}
