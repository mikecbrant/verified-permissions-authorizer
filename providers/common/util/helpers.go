package util

import (
    "os"
    "path/filepath"
)

func StrPtr(s string) *string { return &s }
func Int32Ptr(v int32) *int32 { return &v }

// AbsPath returns p if already absolute, otherwise joins with current working directory.
func AbsPath(p string) string {
    if filepath.IsAbs(p) { return p }
    cwd, _ := os.Getwd()
    return filepath.Join(cwd, p)
}
