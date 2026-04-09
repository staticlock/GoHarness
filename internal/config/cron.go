package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

type CronJob struct {
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
	Command  string `json:"command"`
	Cwd      string `json:"cwd,omitempty"`
}

func CronRegistryPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cron_registry.json"), nil
}

func LoadCronJobs() ([]CronJob, error) {
	path, err := CronRegistryPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []CronJob{}, nil
		}
		return nil, err
	}
	var jobs []CronJob
	if err := json.Unmarshal(data, &jobs); err != nil {
		return []CronJob{}, nil
	}
	return jobs, nil
}

func SaveCronJobs(jobs []CronJob) error {
	path, err := CronRegistryPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func UpsertCronJob(job CronJob) error {
	jobs, err := LoadCronJobs()
	if err != nil {
		return err
	}
	filtered := make([]CronJob, 0)
	for _, existing := range jobs {
		if existing.Name != job.Name {
			filtered = append(filtered, existing)
		}
	}
	filtered = append(filtered, job)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Name < filtered[j].Name
	})
	return SaveCronJobs(filtered)
}

func DeleteCronJob(name string) (bool, error) {
	jobs, err := LoadCronJobs()
	if err != nil {
		return false, err
	}
	filtered := make([]CronJob, 0)
	for _, job := range jobs {
		if job.Name != name {
			filtered = append(filtered, job)
		}
	}
	if len(filtered) == len(jobs) {
		return false, nil
	}
	return true, SaveCronJobs(filtered)
}

func GetCronJob(name string) (*CronJob, error) {
	jobs, err := LoadCronJobs()
	if err != nil {
		return nil, err
	}
	for _, job := range jobs {
		if job.Name == name {
			return &job, nil
		}
	}
	return nil, nil
}
