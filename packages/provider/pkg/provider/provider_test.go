package provider

import (
    "testing"

    "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Basic smoke test that the component can be instantiated with mocks.
func TestAuthorizerConstructs(t *testing.T) {
    t.Parallel()
    err := pulumi.RunErr(func(ctx *pulumi.Context) error {
        _, err := (&AuthorizerWithPolicyStore{}).Construct(ctx, "test", AuthorizerArgs{}, nil)
        return err
    })
    if err != nil {
        t.Fatalf("construct failed: %v", err)
    }
}
