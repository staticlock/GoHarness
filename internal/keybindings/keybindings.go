package keybindings

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func ParseKeybindings(text string) (map[string]string, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return nil, err
	}

	parsed := make(map[string]string)
	for rawKey, value := range data {
		key := string(rawKey)
		valueStr, ok := value.(string)
		if !ok {
			return nil, &KeybindingError{Message: "value must be string"}
		}
		parsed[key] = valueStr
	}
	return parsed, nil
}

func LoadKeybindings(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return ParseKeybindings(string(data))
}

func GetKeybindingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".openharness", "keybindings.json"), nil
}

func ResolveKeybindings(overrides map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range DefaultKeybindings {
		result[k] = v
	}
	for k, v := range overrides {
		result[k] = v
	}
	return result
}

func LoadAllKeybindings() (map[string]string, error) {
	path, err := GetKeybindingsPath()
	if err != nil {
		return nil, err
	}
	loaded, err := LoadKeybindings(path)
	if err != nil {
		return nil, err
	}
	if loaded == nil {
		return DefaultKeybindings, nil
	}
	return ResolveKeybindings(loaded), nil
}

type KeybindingError struct {
	Message string
}

func (e *KeybindingError) Error() string {
	return e.Message
}
