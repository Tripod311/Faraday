package vault

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

type VaultEntry struct {
	Key     string `json:"key"`
	Content string `json:"content"`
}

type Vault struct {
	entries []VaultEntry
}

func (v *Vault) Deserialize(data json.RawMessage) error {
	err := json.Unmarshal(data, &v.entries)

	return err
}

func (v *Vault) Serialize() (json.RawMessage, error) {
	return json.Marshal(v.entries)
}

func (v *Vault) GetField(path string) (string, error) {
	for _, entry := range v.entries {
		if entry.Key == path {
			return entry.Content, nil
		}
	}

	return "", fmt.Errorf("Key not found")
}

func (v *Vault) SetField(path string, content string) error {
	if !ValidKey(path) {
		return fmt.Errorf("invalid key")
	}

	for i := range v.entries {
		if v.entries[i].Key == path {
			v.entries[i].Content = content
			return nil
		}
	}

	v.entries = append(v.entries, VaultEntry{
		Key:     path,
		Content: content,
	})

	return nil
}

func (v *Vault) DeleteField(path string) {
	for i := range v.entries {
		if v.entries[i].Key == path {
			copy(v.entries[i:], v.entries[i+1:])
			v.entries[len(v.entries)-1] = VaultEntry{}
			v.entries = v.entries[:len(v.entries)-1]
			return
		}
	}
}

func (v *Vault) GetNested(ns string) ([]string, error) {
	if strings.Contains(ns, "/") {
		return nil, fmt.Errorf("must be empty string or namespace name")
	}

	res := make(map[string]struct{})

	switch ns {
	case "":
		// return namespaces
		for _, entry := range v.entries {
			entryParsed := strings.Split(entry.Key, "/")
			res[entryParsed[0]] = struct{}{}
		}
	default:
		// return namespace contents
		for _, entry := range v.entries {
			entryParsed := strings.Split(entry.Key, "/")
			if entryParsed[0] == ns {
				res[entryParsed[1]] = struct{}{}
			}
		}
	}

	return slices.Collect(maps.Keys(res)), nil
}

func ValidKey(key string) bool {
	namespace, content, found := strings.Cut(key, "/")
	return found &&
		namespace != "" &&
		content != "" &&
		!strings.Contains(content, "/")
}
