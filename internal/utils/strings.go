package utils

import (
    "encoding/json"
    "io/fs"
    "path/filepath"

    ds "github.com/bmatcuk/doublestar/v4"
)

// NormalizeJSON minifies JSON text for stable equality comparisons; when input is empty returns empty string.
func NormalizeJSON(s string) string {
    if len(s) == 0 {
        return ""
    }
    var v any
    if err := json.Unmarshal([]byte(s), &v); err != nil {
        return s
    }
    b, err := json.Marshal(v)
    if err != nil {
        return s
    }
    return string(b)
}

// GlobRecursive walks base and matches files against a doublestar pattern (supports **).
func GlobRecursive(base, pattern string) ([]string, error) {
    matches := []string{}
    err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }
        if d.IsDir() {
            return nil
        }
        rel, _ := filepath.Rel(base, path)
        ok, err := ds.PathMatch(pattern, rel)
        if err != nil {
            return err
        }
        if ok {
            matches = append(matches, path)
        }
        return nil
    })
    return matches, err
}
