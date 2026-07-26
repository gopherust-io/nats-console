package profiler

import (
	"bytes"
	"errors"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/google/pprof/profile"

	"github.com/gopherust-io/nats-consol/internal/domain"
)

const summaryLimit = 25

var errUnknownProfile = errors.New("unknown profile type")

// ReadRuntimeStats snapshots goroutine count and heap MemStats.
func ReadRuntimeStats() domain.PprofRuntimeStats {
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
