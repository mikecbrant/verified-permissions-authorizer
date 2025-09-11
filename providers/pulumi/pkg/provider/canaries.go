package provider

import (
    "embed"
    "fmt"
    "os"
    "strings"

    aws "github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
    awsvp "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/verifiedpermissions"
    vpapi "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions"
    vpapiTypes "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions/types"
    "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
    "gopkg.in/yaml.v3"
)

//go:embed assets/canaries/*.yaml
var canaryFS embed.FS

// runCombinedCanaries runs provider-resident canaries (base +, when applicable, action-enforcement)
// and merges them with the consumer-provided canary file at path.
func runCombinedCanaries(ctx *pulumi.Context, store *awsvp.PolicyStore, consumerPath string, agMode string) error {
    if ctx.DryRun() {
        ctx.Log.Info("AVP canary: preview mode; skipping canary execution", &pulumi.LogArgs{})
        return nil
    }
    // Load consumer cases (optional)
    var allCases []canaryCase
    if b, err := os.ReadFile(consumerPath); err == nil {
        var doc canaryDoc
        if err := yaml.Unmarshal(b, &doc); err != nil {
            return fmt.Errorf("invalid canary YAML %s: %w", consumerPath, err)
        }
        for _, c := range doc.Cases {
            allCases = append(allCases, fromYamlCase(c))
        }
    }
    // Provider-resident base deny canaries (safe without entity attributes)
    if b, err := canaryFS.ReadFile("assets/canaries/base-deny.yaml"); err == nil {
        var doc canaryDoc
        if err := yaml.Unmarshal(b, &doc); err == nil {
            for _, c := range doc.Cases {
                allCases = append(allCases, fromYamlCase(c))
            }
        }
    }
    // Action-enforcement canaries only when AG mode is not off
    if !strings.EqualFold(agMode, "off") {
        if b, err := canaryFS.ReadFile("assets/canaries/action-enforcement.yaml"); err == nil {
            var doc canaryDoc
            if err := yaml.Unmarshal(b, &doc); err == nil {
                for _, c := range doc.Cases {
                    allCases = append(allCases, fromYamlCase(c))
                }
            }
        }
    }
    if len(allCases) == 0 {
        ctx.Log.Warn("AVP canary: no cases defined (consumer or provider)", &pulumi.LogArgs{})
        return nil
    }

    // Execute inside ApplyT so store ID resolves
    _ = store.ID().ToStringOutput().ApplyT(func(id string) (string, error) {
        region, err := aws.GetRegion(ctx, nil)
        if err != nil {
            return "", fmt.Errorf("failed to get AWS region: %w", err)
        }
        cfg, err := loadAwsConfig(ctx.Context(), region.Name)
        if err != nil {
            return "", err
        }
        client := vpapi.NewFromConfig(cfg)
        for i, c := range allCases {
            p := vpapiTypes.EntityIdentifier{EntityType: &c.PrincipalType, EntityId: &c.PrincipalId}
            r := vpapiTypes.EntityIdentifier{EntityType: &c.ResourceType, EntityId: &c.ResourceId}
            act := c.Action
            // AWS SDK v2: ActionIdentifier uses string fields; ActionType must be the literal "Action".
            actionType := "Action"
            out, err := client.IsAuthorized(ctx.Context(), &vpapi.IsAuthorizedInput{
                PolicyStoreId: &id,
                Principal:     &p,
                Resource:      &r,
                Action:        &vpapiTypes.ActionIdentifier{ActionType: &actionType, ActionId: &act},
            })
            if err != nil {
                return "", fmt.Errorf("canary #%d failed to execute: %w", i+1, err)
            }
            got := string(out.Decision)
            if !strings.EqualFold(got, c.Expect) {
                return "", fmt.Errorf("canary #%d unexpected decision: got %s, want %s (principal=%s:%s, action=%s, resource=%s:%s)", i+1, got, c.Expect, c.PrincipalType, c.PrincipalId, c.Action, c.ResourceType, c.ResourceId)
            }
        }
        ctx.Log.Info(fmt.Sprintf("AVP canary: %d checks passed", len(allCases)), &pulumi.LogArgs{})
        return id, nil
    })
    return nil
}

type canaryDoc struct {
    Cases []yamlCase `yaml:"cases"`
}

type yamlCase struct {
    Principal map[string]string `yaml:"principal"`
    Action    string            `yaml:"action"`
    Resource  map[string]string `yaml:"resource"`
    Expect    string            `yaml:"expect"`
}
type canaryCase struct {
    PrincipalType string
    PrincipalId   string
    Action        string
    ResourceType  string
    ResourceId    string
    Expect        string
}

func fromYamlCase(c yamlCase) canaryCase {
    return canaryCase{
        PrincipalType: c.Principal["entityType"],
        PrincipalId:   c.Principal["entityId"],
        Action:        c.Action,
        ResourceType:  c.Resource["entityType"],
        ResourceId:    c.Resource["entityId"],
        Expect:        c.Expect,
    }
}
