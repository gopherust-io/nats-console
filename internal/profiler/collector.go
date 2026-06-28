package profiler

import (
	"bytes"
	"maps"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/google/pprof/profile"

	"github.com/gopherust-io/nats-consol/internal/domain"
)

const summaryLimit = 25

var continuousProfiles = []string{"heap", "goroutine", "allocs"}

type Options struct {
	Interval       time.Duration
	CPUSlice       time.Duration
	RuntimeHistory int
}

type Collector struct {
	summaries      map[string]domain.PprofProfileSummary
	stop           chan struct{}
	done           chan struct{}
	runtimeHistory []domain.PprofRuntimeStats
	opts           Options
	mu             sync.RWMutex
}

func Start(opts Options) *Collector {
	if opts.Interval <= 0 {
		opts.Interval = 15 * time.Second
	}
	if opts.CPUSlice <= 0 {
		opts.CPUSlice = 5 * time.Second
	}
	if opts.RuntimeHistory <= 0 {
		opts.RuntimeHistory = 60
	}

	c := &Collector{
		opts:      opts,
		summaries: make(map[string]domain.PprofProfileSummary),
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
	go c.loop()
	return c
}

func (c *Collector) Stop() {
	if c == nil {
		return
	}
	close(c.stop)
	<-c.done
}

func (c *Collector) loop() {
	defer close(c.done)
	c.sample()

	ticker := time.NewTicker(c.opts.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
			c.sample()
		}
	}
}

func (c *Collector) sample() {
	runtimeStats := readRuntimeStats()
	updates := make(map[string]domain.PprofProfileSummary, len(continuousProfiles)+1)

	for _, name := range continuousProfiles {
		data, _, err := CollectNamed(name)
		if err != nil {
			continue
		}
		summary, err := Summarize(name, data, 0)
		if err != nil {
			continue
		}
		updates[name] = summary
	}

	cpuData, cpuSecs, err := CollectCPU(int(c.opts.CPUSlice.Seconds()))
	if err == nil {
		if summary, err := Summarize("cpu", cpuData, cpuSecs); err == nil {
			updates["cpu"] = summary
		}
	}

	c.mu.Lock()
	maps.Copy(c.summaries, updates)
	c.runtimeHistory = append(c.runtimeHistory, runtimeStats)
	if len(c.runtimeHistory) > c.opts.RuntimeHistory {
		c.runtimeHistory = c.runtimeHistory[len(c.runtimeHistory)-c.opts.RuntimeHistory:]
	}
	c.mu.Unlock()
}

func (c *Collector) Snapshot() domain.PprofContinuousSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	profiles := make(map[string]domain.PprofProfileSummary, len(c.summaries))
	for name, summary := range c.summaries {
		if summary.Entries == nil {
			summary.Entries = []domain.PprofProfileEntry{}
		}
		profiles[name] = summary
	}
	history := append([]domain.PprofRuntimeStats(nil), c.runtimeHistory...)
	if history == nil {
		history = []domain.PprofRuntimeStats{}
	}

	return domain.PprofContinuousSnapshot{
		FetchedAt:      time.Now().UTC(),
		IntervalSecs:   int(c.opts.Interval.Seconds()),
		CPUSliceSecs:   int(c.opts.CPUSlice.Seconds()),
		Profiles:       profiles,
		RuntimeHistory: history,
	}
}

func (c *Collector) Profile(name string) (domain.PprofProfileSummary, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	summary, ok := c.summaries[name]
	return summary, ok
}

func readRuntimeStats() domain.PprofRuntimeStats {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return domain.PprofRuntimeStats{
		FetchedAt:  time.Now().UTC(),
		Goroutines: runtime.NumGoroutine(),
		Memory: domain.PprofMemoryStats{
			Alloc:       stats.Alloc,
			TotalAlloc:  stats.TotalAlloc,
			Sys:         stats.Sys,
			HeapAlloc:   stats.HeapAlloc,
			HeapInuse:   stats.HeapInuse,
			HeapObjects: stats.HeapObjects,
			NumGC:       stats.NumGC,
		},
	}
}

func CollectCPU(seconds int) ([]byte, int, error) {
	if seconds <= 0 {
		seconds = 5
	}
	var buf bytes.Buffer
	if err := pprof.StartCPUProfile(&buf); err != nil {
		return nil, 0, err
	}
	time.Sleep(time.Duration(seconds) * time.Second)
	pprof.StopCPUProfile()
	return buf.Bytes(), seconds, nil
}

func CollectNamed(name string) ([]byte, int, error) {
	prof := pprof.Lookup(name)
	if prof == nil {
		return nil, 0, errUnknownProfile
	}
	var buf bytes.Buffer
	if err := prof.WriteTo(&buf, 0); err != nil {
		return nil, 0, err
	}
	return buf.Bytes(), 0, nil
}

func Summarize(profileType string, data []byte, durationSecs int) (domain.PprofProfileSummary, error) {
	prof, err := profile.Parse(bytes.NewReader(data))
	if err != nil {
		return domain.PprofProfileSummary{}, err
	}

	type agg struct {
		name string
		flat int64
		cum  int64
	}
	byName := make(map[string]*agg)
	var totalFlat int64

	for _, sample := range prof.Sample {
		var flat int64
		if len(sample.Value) > 0 {
			flat = sample.Value[0]
		}
		var cum int64
		if len(sample.Value) > 1 {
			cum = sample.Value[1]
		} else {
			cum = flat
		}
		if flat == 0 && cum == 0 {
			continue
		}

		name := sampleFunctionName(sample)
		entry := byName[name]
		if entry == nil {
			entry = &agg{name: name}
			byName[name] = entry
		}
		entry.flat += flat
		entry.cum += cum
		totalFlat += flat
	}

	entries := make([]domain.PprofProfileEntry, 0, len(byName))
	for _, item := range byName {
		entry := domain.PprofProfileEntry{
			Name: item.name,
			Flat: item.flat,
			Cum:  item.cum,
		}
		if totalFlat > 0 {
			entry.FlatPercent = float64(item.flat) * 100 / float64(totalFlat)
			entry.CumPercent = float64(item.cum) * 100 / float64(totalFlat)
		}
		entries = append(entries, entry)
	}

	sortProfileEntries(entries)
	if len(entries) > summaryLimit {
		entries = entries[:summaryLimit]
	}

	return domain.PprofProfileSummary{
		FetchedAt:    time.Now().UTC(),
		ProfileType:  profileType,
		TotalSamples: totalFlat,
		DurationSecs: durationSecs,
		Entries:      entries,
	}, nil
}

func sampleFunctionName(sample *profile.Sample) string {
	if len(sample.Location) == 0 {
		return "unknown"
	}
	loc := sample.Location[0]
	if len(loc.Line) == 0 {
		return "unknown"
	}
	fn := loc.Line[0].Function
	if fn == nil {
		return "unknown"
	}
	return fn.Name
}

func sortProfileEntries(entries []domain.PprofProfileEntry) {
	for i := 1; i < len(entries); i++ {
		j := i
		for j > 0 && entries[j].Flat > entries[j-1].Flat {
			entries[j], entries[j-1] = entries[j-1], entries[j]
			j--
		}
	}
}
