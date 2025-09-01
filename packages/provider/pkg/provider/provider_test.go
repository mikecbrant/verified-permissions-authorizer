package provider

import (
    "testing"

    "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
    "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type testMocks struct{}

func (testMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
    // Echo inputs as outputs; synthesize an ID
    return args.Name + "_id", args.Inputs, nil
}

func (testMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
    return resource.PropertyMap{}, nil
}

// Basic smoke test that the component can be instantiated with mocks.
func TestAuthorizerConstructs(t *testing.T) {
    t.Parallel()
    err := pulumi.RunErr(func(ctx *pulumi.Context) error {
        _, err := NewAuthorizerWithPolicyStore(ctx, "test", AuthorizerArgs{})
        return err
    }, pulumi.WithMocks("test", "dev", testMocks{}))
    if err != nil {
        t.Fatalf("construct failed: %v", err)
    }
}
