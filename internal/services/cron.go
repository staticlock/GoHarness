package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

func getCronRegistryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".openharness")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "cron.json"), nil
}

type CronJob map[string]interface{}

func LoadCronJobs() ([]CronJob, error) {
	path, err := getCronRegistryPath()
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
	path, err := getCronRegistryPath()
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
	name, _ := job["name"].(string)
	jobs, err := LoadCronJobs()
	if err != nil {
		return err
	}
	var filtered []CronJob
	for _, existing := range jobs {
		existingName, _ := existing["name"].(string)
		if existingName != name {
			filtered = append(filtered, existing)
		}
	}
	filtered = append(filtered, job)
	sort.Slice(filtered, func(i, j int) bool {
		iName, _ := filtered[i]["name"].(string)
		jName, _ := filtered[j]["name"].(string)
		return iName < jName
	})
	return SaveCronJobs(filtered)
}

func DeleteCronJob(name string) (bool, error) {
	jobs, err := LoadCronJobs()
	if err != nil {
		return false, err
	}
	var filtered []CronJob
	for _, job := range jobs {
		jobName, _ := job["name"].(string)
		if jobName != name {
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
		jobName, _ := job["name"].(string)
		if jobName == name {
			return &job, nil
		}
	}
	return nil, nil
}
