package provider

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

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
