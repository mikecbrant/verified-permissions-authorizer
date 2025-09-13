package main

import (
	"context"
	"fmt"
	"os"

	provider "github.com/mikecbrant/verified-permissions-authorizer/internal/pulumi"
	p "github.com/pulumi/pulumi-go-provider"
)

func main() {
	prov, err := provider.NewProvider()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := p.RunProvider(context.Background(), "verified-permissions-authorizer", "0.0.0", prov); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
