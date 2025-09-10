package avp

import (
    "context"
    "embed"
    "fmt"
    "os"
    "strings"

    vpapi "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions"
    vpapiTypes "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions/types"
    "github.com/mikecbrant/verified-permissions-authorizer/providers/internal/awsutil"
    "gopkg.in/yaml.v3"
)

//go:embed assets/canaries/*.yaml
var canaryFS embed.FS

type yamlCase struct {
    Principal map[string]string `yaml:"principal"`
    Action    string            `yaml:"action"`
    Resource  map[string]string `yaml:"resource"`
    Expect    string            `yaml:"expect"`
}
type canaryDoc struct {
    Cases []yamlCase `yaml:"cases"`
}

// RunCombinedCanaries merges provider-resident canaries with an optional consumer canary file
// and executes them against the policy store. If agMode is "off", the action-enforcement
// canaries are skipped.
func RunCombinedCanaries(ctx context.Context, region string, policyStoreId string, consumerPath string, agMode string) error {
    cfg, err := awsutil.LoadDefault(ctx, region)
    if err != nil {
        return err
    }
    client := vpapi.NewFromConfig(cfg)

    type canaryCase struct {
        PrincipalType string
        PrincipalId   string
        Action        string
        ResourceType  string
        ResourceId    string
        Expect        string
    }
    toCase := func(c yamlCase) canaryCase {
        return canaryCase{
            PrincipalType: c.Principal["entityType"],
            PrincipalId:   c.Principal["entityId"],
            Action:        c.Action,
            ResourceType:  c.Resource["entityType"],
            ResourceId:    c.Resource["entityId"],
            Expect:        c.Expect,
        }
    }

    var allCases []canaryCase
    if b, err := os.ReadFile(consumerPath); err == nil {
        var doc canaryDoc
        if err := yaml.Unmarshal(b, &doc); err != nil {
            return fmt.Errorf("invalid canary YAML %s: %w", consumerPath, err)
        }
        for _, c := range doc.Cases {
            allCases = append(allCases, toCase(c))
        }
    }
    if b, err := canaryFS.ReadFile("assets/canaries/base-deny.yaml"); err == nil {
        var doc canaryDoc
        if err := yaml.Unmarshal(b, &doc); err == nil {
            for _, c := range doc.Cases {
                allCases = append(allCases, toCase(c))
            }
        }
    }
    if !strings.EqualFold(agMode, "off") {
        if b, err := canaryFS.ReadFile("assets/canaries/action-enforcement.yaml"); err == nil {
            var doc canaryDoc
            if err := yaml.Unmarshal(b, &doc); err == nil {
                for _, c := range doc.Cases {
                    allCases = append(allCases, toCase(c))
                }
            }
        }
    }
    if len(allCases) == 0 {
        return nil
    }
    for i, c := range allCases {
        p := vpapiTypes.EntityIdentifier{EntityType: &c.PrincipalType, EntityId: &c.PrincipalId}
        r := vpapiTypes.EntityIdentifier{EntityType: &c.ResourceType, EntityId: &c.ResourceId}
        act := c.Action
        actionType := "Action"
        out, err := client.IsAuthorized(ctx, &vpapi.IsAuthorizedInput{
            PolicyStoreId: &policyStoreId,
            Principal:     &p,
            Resource:      &r,
            Action:        &vpapiTypes.ActionIdentifier{ActionType: &actionType, ActionId: &act},
        })
        if err != nil {
            return fmt.Errorf("canary #%d failed to execute: %w", i+1, err)
        }
        got := string(out.Decision)
        if !strings.EqualFold(got, c.Expect) {
            return fmt.Errorf("canary #%d unexpected decision: got %s, want %s (principal=%s:%s, action=%s, resource=%s:%s)", i+1, got, c.Expect, c.PrincipalType, c.PrincipalId, c.Action, c.ResourceType, c.ResourceId)
        }
    }
    return nil
}
