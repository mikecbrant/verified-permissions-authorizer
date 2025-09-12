package util

import (
    "embed"
    "fmt"
    "io/fs"
    "os"

    "gopkg.in/yaml.v3"
)

// ReadYAML reads a YAML file from disk into the provided generic type.
func ReadYAML[T any](path string, out *T) error {
    b, err := os.ReadFile(path)
    if err != nil { return err }
    if err := yaml.Unmarshal(b, out); err != nil {
        return fmt.Errorf("invalid YAML %s: %w", path, err)
    }
    return nil
}

// ReadYAMLFromFS reads a YAML file from an embedded FS into the provided generic type.
func ReadYAMLFromFS[T any](efs embed.FS, name string, out *T) error {
    b, err := fs.ReadFile(efs, name)
    if err != nil { return err }
    if err := yaml.Unmarshal(b, out); err != nil {
        return fmt.Errorf("invalid embedded YAML %s: %w", name, err)
    }
    return nil
}
