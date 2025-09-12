package avp

import (
    "context"
    "embed"
    "fmt"
    "strings"

    vpapi "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions"
    vpapiTypes "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions/types"
    "github.com/mikecbrant/verified-permissions-authorizer/providers/common/awsutil"
    sharedutil "github.com/mikecbrant/verified-permissions-authorizer/providers/common/util"
)

//go:embed assets/canaries/*.yaml
var canaryFS embed.FS

type yamlCase struct {
    Principal map[string]string `yaml:"principal"`
    Action    string            `yaml:"action"`
    Resource  map[string]string `yaml:"resource"`
    Expect    string            `yaml:"expect"`
    Context   map[string]any    `yaml:"context"`
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
        Context       map[string]any
    }
    toCase := func(c yamlCase) canaryCase {
        return canaryCase{
            PrincipalType: c.Principal["entityType"],
            PrincipalId:   c.Principal["entityId"],
            Action:        c.Action,
            ResourceType:  c.Resource["entityType"],
            ResourceId:    c.Resource["entityId"],
            Expect:        c.Expect,
            Context:       c.Context,
        }
    }

    var allCases []canaryCase
    if consumerPath != "" {
        var doc canaryDoc
        if err := sharedutil.ReadYAML(consumerPath, &doc); err != nil {
            return fmt.Errorf("invalid canary YAML %s: %w", consumerPath, err)
        }
        for _, c := range doc.Cases { allCases = append(allCases, toCase(c)) }
    }
    {
        var doc canaryDoc
        if err := sharedutil.ReadYAMLFromFS(canaryFS, "assets/canaries/base-deny.yaml", &doc); err == nil {
            for _, c := range doc.Cases { allCases = append(allCases, toCase(c)) }
        }
    }
    if !strings.EqualFold(agMode, "off") {
        var doc canaryDoc
        if err := sharedutil.ReadYAMLFromFS(canaryFS, "assets/canaries/action-enforcement.yaml", &doc); err == nil {
            for _, c := range doc.Cases { allCases = append(allCases, toCase(c)) }
        }
    }
    if len(allCases) == 0 {
        return nil
    }
    var failures []string
    for i, c := range allCases {
        p := vpapiTypes.EntityIdentifier{EntityType: &c.PrincipalType, EntityId: &c.PrincipalId}
        r := vpapiTypes.EntityIdentifier{EntityType: &c.ResourceType, EntityId: &c.ResourceId}
        act := c.Action
        actionType := "Action"
        in := &vpapi.IsAuthorizedInput{
            PolicyStoreId: &policyStoreId,
            Principal:     &p,
            Resource:      &r,
            Action:        &vpapiTypes.ActionIdentifier{ActionType: &actionType, ActionId: &act},
        }
        if len(c.Context) > 0 {
            cm := map[string]vpapiTypes.AttributeValue{}
            for k, v := range c.Context {
                switch t := v.(type) {
                case bool:
                    cm[k] = &vpapiTypes.AttributeValueMemberBoolean{Value: t}
                case int:
                    cm[k] = &vpapiTypes.AttributeValueMemberLong{Value: int64(t)}
                case int64:
                    cm[k] = &vpapiTypes.AttributeValueMemberLong{Value: t}
                case string:
                    cm[k] = &vpapiTypes.AttributeValueMemberString{Value: t}
                default:
                    s := fmt.Sprint(v)
                    cm[k] = &vpapiTypes.AttributeValueMemberString{Value: s}
                }
            }
            in.Context = &vpapiTypes.ContextDefinitionMemberContextMap{Value: cm}
        }
        out, err := client.IsAuthorized(ctx, in)
        if err != nil {
            failures = append(failures, fmt.Sprintf("%d: API error: %v", i+1, err))
            continue
        }
        got := "DENY"
        if out.Decision == vpapiTypes.DecisionAllow { got = "ALLOW" }
        if !strings.EqualFold(got, c.Expect) {
            failures = append(failures, fmt.Sprintf("%d: expected %s, got %s (principal=%s:%s, action=%s, resource=%s:%s)", i+1, c.Expect, got, c.PrincipalType, c.PrincipalId, c.Action, c.ResourceType, c.ResourceId))
        }
    }
    if len(failures) > 0 {
        return fmt.Errorf("canaries failed (%d/%d): %s", len(failures), len(allCases), strings.Join(failures, "; "))
    }
    return nil
}
