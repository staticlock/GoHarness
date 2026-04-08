package hooks

// Result is the execution output for one hook.
type Result struct {
	HookType string
	Success  bool
	Output   string
	Blocked  bool
	Reason   string
	Metadata map[string]any
}

// AggregatedResult is the merged output for all hooks on one event.
type AggregatedResult struct {
	Results []Result
}

// IsBlocked reports whether any hook blocked continuation.
func (a AggregatedResult) IsBlocked() bool {
	for _, r := range a.Results {
		if r.Blocked {
			return true
		}
	}
	return false
}

// Reason returns the first blocking reason.
func (a AggregatedResult) Reason() string {
	for _, r := range a.Results {
		if r.Blocked {
			if r.Reason != "" {
				return r.Reason
			}
			return r.Output
		}
	}
	return ""
}
