package common

import (
	"context"
	"embed"
	"fmt"
	"os"
	"strings"

	vpapi "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions"
	vpapiTypes "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions/types"
	"gopkg.in/yaml.v3"

	"github.com/mikecbrant/verified-permissions-authorizer/internal/awssdk"
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

type canaryCase struct {
	PrincipalType string
	PrincipalId   string
	Action        string
	ResourceType  string
	ResourceId    string
	Expect        string
}

func toCanaryCase(c yamlCase) canaryCase {
	return canaryCase{
		PrincipalType: c.Principal["entityType"],
		PrincipalId:   c.Principal["entityId"],
		Action:        c.Action,
		ResourceType:  c.Resource["entityType"],
		ResourceId:    c.Resource["entityId"],
		Expect:        c.Expect,
	}
}

// RunCombinedCanaries merges provider-resident canaries with an optional consumer canary file
// and executes them against the policy store. If agMode is "off", the action-enforcement
// canaries are skipped.
func RunCombinedCanaries(ctx context.Context, region string, policyStoreId string, consumerPath string, agMode string) error {
	cfg, err := awssdk.LoadDefault(ctx, region)
	if err != nil {
		return err
	}
	client := vpapi.NewFromConfig(cfg)

	allCases, err := loadCanaryCases(consumerPath, agMode)
	if err != nil {
		return err
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

func loadCanaryCases(consumerPath string, agMode string) ([]canaryCase, error) {
	allCases := []canaryCase{}
	if b, err := os.ReadFile(consumerPath); err == nil {
		doc, err := readCanaryDoc(b, consumerPath)
		if err != nil {
			return nil, err
		}
		for _, c := range doc.Cases {
			allCases = append(allCases, toCanaryCase(c))
		}
	}

	baseCases := readEmbeddedCanaryCases("assets/canaries/base-deny.yaml")
	allCases = append(allCases, baseCases...)

	if !strings.EqualFold(agMode, "off") {
		agCases := readEmbeddedCanaryCases("assets/canaries/action-enforcement.yaml")
		allCases = append(allCases, agCases...)
	}

	return allCases, nil
}

func readEmbeddedCanaryCases(assetPath string) []canaryCase {
	b, err := canaryFS.ReadFile(assetPath)
	if err != nil {
		return nil
	}
	doc, err := readCanaryDoc(b, assetPath)
	if err != nil {
		return nil
	}
	cases := make([]canaryCase, 0, len(doc.Cases))
	for _, c := range doc.Cases {
		cases = append(cases, toCanaryCase(c))
	}
	return cases
}

func readCanaryDoc(b []byte, src string) (canaryDoc, error) {
	var doc canaryDoc
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return canaryDoc{}, fmt.Errorf("invalid canary YAML %s: %w", src, err)
	}
	return doc, nil
}
