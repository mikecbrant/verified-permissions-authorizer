package main

import (
    "log"

    prov "github.com/mikecbrant/verified-permissions-authorizer/provider/pkg/provider"
    p "github.com/pulumi/pulumi-go-provider"
)

func main() {
    var err error
    var srv p.Provider
    srv, err = prov.NewProvider()
    if err != nil {
        log.Fatalf("failed to create provider: %v", err)
    }
    if err := p.Serve("verified-permissions-authorizer", srv); err != nil {
        log.Fatalf("provider serve failed: %v", err)
    }
}
