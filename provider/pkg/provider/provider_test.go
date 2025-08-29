package provider

import (
    "testing"

    "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// testMonitor implements pulumi.MockResourceMonitor to run the construct offline.
type testMonitor struct{}

func (m testMonitor) NewResource(args pulumi.MockResourceArgs) (string, map[string]interface{}, error) {
    // Return synthetic IDs and minimal properties referenced by the component.
    state := map[string]interface{}{
        "arn":  args.Name + ":arn",
        "name": args.Name,
        "id":   args.Name + ":id",
    }
    return args.Name + "-id", state, nil
}

func (m testMonitor) Call(args pulumi.MockCallArgs) (map[string]interface{}, error) {
    // Used by aws.iam.getPolicyDocument
    return map[string]interface{}{"json": "{}"}, nil
}

func TestNewProvider(t *testing.T) {
    if _, err := NewProvider(); err != nil {
        t.Fatalf("NewProvider error: %v", err)
    }
}

func TestConstructAuthorizerWithPolicyStore(t *testing.T) {
    err := pulumi.RunErr(func(ctx *pulumi.Context) error {
        comp := &AuthorizerWithPolicyStore{}
        desc := "test store"
        mode := "STRICT"
        _, err := comp.Construct(ctx, "example", AuthorizerArgs{
            Description:   &desc,
            ValidationMode: &mode,
            LambdaEnv:      map[string]string{"FOO": "bar"},
        }, nil)
        return err
    }, pulumi.WithMocks("proj", "stack", testMonitor{}))
    if err != nil {
        t.Fatalf("Construct failed: %v", err)
    }
}
