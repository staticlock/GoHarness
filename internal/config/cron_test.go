package config

import (
	"os"
	"strings"
	"testing"
)

func TestCronJobOperations(t *testing.T) {
	tmp := t.TempDir()
	originalConfigDir := os.Getenv("OPENHARNESS_CONFIG_DIR")
	os.Setenv("OPENHARNESS_CONFIG_DIR", tmp)
	defer os.Unsetenv("OPENHARNESS_CONFIG_DIR")
	if originalConfigDir != "" {
		os.Setenv("OPENHARNESS_CONFIG_DIR", originalConfigDir)
	}

	job1 := CronJob{Name: "job1", Schedule: "daily", Command: "echo hello"}
	err := UpsertCronJob(job1)
	if err != nil {
		t.Fatalf("UpsertCronJob failed: %v", err)
	}

	jobs, err := LoadCronJobs()
	if err != nil {
		t.Fatalf("LoadCronJobs failed: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Name != "job1" {
		t.Fatalf("unexpected jobs: %+v", jobs)
	}

	job2 := CronJob{Name: "job2", Schedule: "hourly", Command: "echo world"}
	UpsertCronJob(job2)

	jobs, _ = LoadCronJobs()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	deleted, err := DeleteCronJob("job1")
	if err != nil || !deleted {
		t.Fatalf("DeleteCronJob failed")
	}

	jobs, _ = LoadCronJobs()
	if len(jobs) != 1 || jobs[0].Name != "job2" {
		t.Fatalf("unexpected after delete: %+v", jobs)
	}

	found, _ := GetCronJob("job2")
	if found == nil || found.Command != "echo world" {
		t.Fatalf("GetCronJob failed: %+v", found)
	}
}

func TestCronRegistryPath(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("OPENHARNESS_CONFIG_DIR", tmp)
	defer os.Unsetenv("OPENHARNESS_CONFIG_DIR")

	path, err := CronRegistryPath()
	if err != nil {
		t.Fatalf("CronRegistryPath failed: %v", err)
	}
	if !strings.HasSuffix(path, "cron_registry.json") {
		t.Fatalf("unexpected path: %s", path)
	}
}
