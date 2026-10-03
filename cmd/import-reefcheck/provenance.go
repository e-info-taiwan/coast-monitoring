package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

func importSourceSnapshot(dir string) (string, []byte, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.csv"))
	if err != nil {
		return "", nil, err
	}
	sort.Strings(paths)
	snapshot := map[string]string{}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", nil, err
		}
		snapshot[filepath.Base(path)] = string(raw)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), raw, nil
}
