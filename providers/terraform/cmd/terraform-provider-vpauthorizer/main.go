package main

import (
    "context"
    "flag"
    "log"

    "github.com/hashicorp/terraform-plugin-framework/providerserver"
    "github.com/mikecbrant/verified-permissions-authorizer/providers/terraform/internal/provider"
)

var (
    // Set via -ldflags in release
    version = "dev"
)

func main() {
    var debug bool
    flag.BoolVar(&debug, "debug", false, "enable debug mode")
    flag.Parse()
    opts := providerserver.ServeOpts{
        Address:         "registry.terraform.io/mikecbrant/vpauthorizer",
        Debug:           debug,
        ProtocolVersion: 6,
    }
    if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
        log.Fatal(err)
    }
}
