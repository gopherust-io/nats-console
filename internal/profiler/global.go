package profiler

import "errors"

var errUnknownProfile = errors.New("unknown profile type")

// Default is the process-wide continuous profiler started when PPROF_CONTINUOUS is enabled.
var Default *Collector

func StartDefault(opts Options) *Collector {
	if Default != nil {
		Default.Stop()
	}
	Default = Start(opts)
	return Default
}

func StopDefault() {
	if Default != nil {
		Default.Stop()
		Default = nil
	}
}
