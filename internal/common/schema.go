package common

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	vpapi "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions"
	vpapiTypes "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions/types"
	"gopkg.in/yaml.v3"

	"github.com/mikecbrant/verified-permissions-authorizer/internal/awssdk"
	"github.com/mikecbrant/verified-permissions-authorizer/internal/utils"
)

var namespaceNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]+$`)

var requiredPrincipals = []string{"Tenant", "User", "Role", "GlobalRole", "TenantGrant"}

// LoadAndValidateSchema parses a YAML/JSON Verified Permissions schema definition and returns
// canonical JSON (minified), the namespace name, the set of action names, and any warnings.
func LoadAndValidateSchema(schemaPath string) (cedarJSON string, namespace string, actions []string, warnings []string, err error) {
	doc, err := loadSchemaDocument(schemaPath)
	if err != nil {
		return "", "", nil, nil, err
	}

	top, ns, body, err := extractSingleNamespace(doc)
	if err != nil {
		return "", "", nil, nil, err
	}

	warnings = namespaceWarnings(ns)

	acts, err := collectActionNames(body)
	if err != nil {
		return "", "", nil, nil, err
	}

	if err := validateRequiredPrincipals(ns, body); err != nil {
		return "", "", nil, nil, err
	}

	cedarJSON, err = canonicalizeSchema(top)
	if err != nil {
		return "", "", nil, nil, err
	}

	return cedarJSON, ns, acts, warnings, nil
}

func loadSchemaDocument(schemaPath string) (any, error) {
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
	}

	var doc any
	switch strings.ToLower(filepath.Ext(schemaPath)) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("invalid YAML in %s: %w", schemaPath, err)
		}
	case ".json":
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("invalid JSON in %s: %w", schemaPath, err)
		}
	default:
		return nil, fmt.Errorf("unsupported schema extension %q; expected .yaml, .yml, or .json", filepath.Ext(schemaPath))
	}

	return doc, nil
}

func extractSingleNamespace(doc any) (top map[string]any, ns string, body map[string]any, err error) {
	// Expect single namespace object
	top, ok := doc.(map[string]any)
	if !ok {
		return nil, "", nil, fmt.Errorf("schema must be a mapping of namespace → {entityTypes, actions}")
	}
	if len(top) != 1 {
		return nil, "", nil, fmt.Errorf("AVP supports a single namespace per schema; found %d namespaces", len(top))
	}
	for k, v := range top {
		ns = k
		b, ok := v.(map[string]any)
		if !ok {
			return nil, "", nil, fmt.Errorf("schema namespace %q must map to an object", ns)
		}
		return top, ns, b, nil
	}
	return nil, "", nil, fmt.Errorf("schema must contain exactly one namespace")
}

func namespaceWarnings(ns string) []string {
	// Warning only; provider may elevate this to error.
	if namespaceNameRe.MatchString(ns) {
		return nil
	}
	return []string{fmt.Sprintf("namespace %q is non-standard; consider simple kebab-case", ns)}
}

func validateRequiredPrincipals(ns string, body map[string]any) error {
	etRaw, ok := body["entityTypes"]
	if !ok {
		return fmt.Errorf("schema namespace %q must define entityTypes", ns)
	}
	et, ok := etRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("entityTypes must be an object of entity type definitions")
	}
	missing := []string{}
	for _, r := range requiredPrincipals {
		if _, ok := et[r]; !ok {
			missing = append(missing, r)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("schema namespace %q missing required principal entity types: %s", ns, strings.Join(missing, ", "))
	}
	return nil
}

func collectActionNames(body map[string]any) ([]string, error) {
	acts := []string{}
	aRaw, ok := body["actions"]
	if !ok {
		return acts, nil
	}
	amap, ok := aRaw.(map[string]any)
	if !ok {
		return acts, nil
	}
	for name := range amap {
		acts = append(acts, name)
	}
	return acts, nil
}

func canonicalizeSchema(top map[string]any) (string, error) {
	b, err := json.Marshal(top)
	if err != nil {
		return "", fmt.Errorf("failed to encode schema as JSON: %w", err)
	}
	if sz := len(b); sz > 100000 {
		return "", fmt.Errorf("schema JSON size %d exceeds 100,000 byte limit", sz)
	}
	return string(b), nil
}

// Canonical action group identifiers (PascalCase + Global* variants)
var canonicalActionGroups = []string{
	"BatchCreate", "Create", "BatchDelete", "Delete", "Find", "Get", "BatchUpdate", "Update",
	"GlobalBatchCreate", "GlobalCreate", "GlobalBatchDelete", "GlobalDelete", "GlobalFind", "GlobalGet", "GlobalBatchUpdate", "GlobalUpdate",
}

// EnforceActionGroups checks action names map to canonical groups via exact, case-sensitive prefixes.
// mode: "off" | "warn" | "error". Returns violating action names and (when mode==error) an error.
func EnforceActionGroups(actions []string, mode string) ([]string, error) {
	if strings.EqualFold(mode, "off") {
		return nil, nil
	}
	bad := []string{}
	for _, a := range actions {
		ok := false
		for _, g := range canonicalActionGroups {
			if strings.HasPrefix(a, g) {
				ok = true
				break
			}
		}
		if !ok {
			bad = append(bad, a)
		}
	}
	if len(bad) == 0 {
		return nil, nil
	}
	if mode == "error" {
		return bad, fmt.Errorf("actions not aligned to canonical action groups %v: %s", canonicalActionGroups, strings.Join(bad, ", "))
	}
	return bad, nil
}

// PutSchemaIfChanged fetches the current schema and applies only when content differs.
func PutSchemaIfChanged(ctx context.Context, policyStoreId string, cedarJSON string, region string) error {
	cfg, err := awssdk.LoadDefault(ctx, region)
	if err != nil {
		return err
	}
	client := vpapi.NewFromConfig(cfg)
	var current string
	getOut, err := client.GetSchema(ctx, &vpapi.GetSchemaInput{PolicyStoreId: &policyStoreId})
	if err == nil && getOut.Schema != nil {
		current = *getOut.Schema
	}
	if utils.NormalizeJSON(current) == utils.NormalizeJSON(cedarJSON) {
		return nil
	}
	_, err = client.PutSchema(ctx, &vpapi.PutSchemaInput{
		PolicyStoreId: &policyStoreId,
		Definition:    &vpapiTypes.SchemaDefinitionMemberCedarJson{Value: cedarJSON},
	})
	if err != nil {
		return fmt.Errorf("failed to put schema: %w", err)
	}
	return nil
}

// CollectPolicyFiles returns deterministic list of .cedar policy files under dir.
func CollectPolicyFiles(dir string) ([]string, error) {
	files, err := utils.GlobRecursive(dir, "**/*.cedar")
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
