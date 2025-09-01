package main

import (
    "fmt"
    "os"

    "github.com/mikecbrant/verified-permissions-authorizer/provider/pkg/provider"
    p "github.com/pulumi/pulumi-go-provider"
)

func main() {
    prov, err := provider.NewProvider()
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
    if err := p.RunProvider("verified-permissions-authorizer", "0.0.0", prov); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
