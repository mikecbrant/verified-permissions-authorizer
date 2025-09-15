package provider

import (
	"io/fs"
	"path/filepath"

	ds "github.com/bmatcuk/doublestar/v4"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// globRecursive implements a simple recursive glob: base + pattern (supports **).
func globRecursive(base, pattern string) ([]string, error) {
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

func toOutputs(ins []pulumi.StringOutput) []pulumi.Output {
	outs := make([]pulumi.Output, 0, len(ins))
	for _, in := range ins {
		outs = append(outs, in)
	}
	return outs
}

// outputsToInterfaces converts a slice of pulumi.Output to a slice of interface{}
// suitable for passing to variadic functions like pulumi.All.
func outputsToInterfaces(ins []pulumi.Output) []interface{} {
	out := make([]interface{}, len(ins))
	for i, v := range ins {
		out[i] = v
	}
	return out
}

func valueOrDefault[T ~string](ptr *T, def T) string { // generic-ish helper for *string
	if ptr == nil {
		return string(def)
	}
	return string(*ptr)
}
