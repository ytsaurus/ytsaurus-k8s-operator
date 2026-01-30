package resources

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// Hash returns hex sha256 for concatenated json serialization.
// Excludes trailing nil, returns "" if there is nothing left.
func Hash(items ...any) (string, error) {
	for len(items) > 0 && items[len(items)-1] == nil {
		items = items[:len(items)-1]
	}
	if len(items) == 0 {
		return "", nil
	}
	hash := sha256.New()
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			return "", err
		}
		_, err = hash.Write(data)
		if err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
