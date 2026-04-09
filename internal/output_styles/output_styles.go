package output_styles

import (
	"os"
	"path/filepath"
	"sort"
)

type OutputStyle struct {
	Name    string
	Content string
	Source  string
}

func GetOutputStylesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".openharness", "output_styles")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func LoadOutputStyles() ([]OutputStyle, error) {
	styles := []OutputStyle{
		{Name: "default", Content: "Standard rich console output.", Source: "builtin"},
		{Name: "minimal", Content: "Very terse plain-text output.", Source: "builtin"},
	}

	dir, err := GetOutputStylesDir()
	if err != nil {
		return styles, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return styles, nil
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		name := entry.Name()[:len(entry.Name())-3]
		styles = append(styles, OutputStyle{
			Name:    name,
			Content: string(data),
			Source:  "user",
		})
	}

	sort.Slice(styles, func(i, j int) bool {
		if styles[i].Source != styles[j].Source {
			return styles[i].Source == "builtin"
		}
		return styles[i].Name < styles[j].Name
	})

	return styles, nil
}
