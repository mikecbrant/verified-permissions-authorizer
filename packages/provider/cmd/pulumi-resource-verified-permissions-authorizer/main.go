package main

import (
    "github.com/mikecbrant/verified-permissions-authorizer/provider/pkg/provider"
    p "github.com/pulumi/pulumi-go-provider"
)

func main() {
    p.RunProvider("verified-permissions-authorizer", func() (p.Provider, error) {
        return provider.NewProvider()
    })
}
