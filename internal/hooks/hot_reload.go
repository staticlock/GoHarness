package hooks

import (
	"os"
)

type Reloader struct {
	settingsPath string
	lastMtime    int64
	registry     *Registry
}

func NewReloader(settingsPath string) *Reloader {
	return &Reloader{
		settingsPath: settingsPath,
		lastMtime:    -1,
		registry:     NewRegistry(),
	}
}

func (r *Reloader) CurrentRegistry() *Registry {
	stat, err := os.Stat(r.settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			r.lastMtime = -1
			r.registry = NewRegistry()
			return r.registry
		}
		return r.registry
	}

	currentMtime := stat.ModTime().UnixNano()
	if currentMtime != r.lastMtime {
		r.lastMtime = currentMtime
		r.registry = NewRegistry()
	}
	return r.registry
}
