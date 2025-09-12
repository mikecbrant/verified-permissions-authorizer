package avp

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
    "github.com/mikecbrant/verified-permissions-authorizer/providers/common/awsutil"
    "github.com/mikecbrant/verified-permissions-authorizer/providers/common/util"
    "gopkg.in/yaml.v3"
)

// LoadAndValidateSchema parses a YAML/JSON Verified Permissions schema definition and returns
// canonical JSON (minified), the namespace name, the set of action names, and any warnings.
func LoadAndValidateSchema(schemaPath string) (cedarJSON string, namespace string, actions []string, warnings []string, err error) {
    return LoadAndValidateSchemaWithNamespace(schemaPath, "")
}

// LoadAndValidateSchemaWithNamespace is like LoadAndValidateSchema but allows the caller to override
// the namespace (top-level key) with a provided value. When override is non-empty it must pass strict
// namespace validation, otherwise an error is returned.
func LoadAndValidateSchemaWithNamespace(schemaPath string, overrideNamespace string) (cedarJSON string, namespace string, actions []string, warnings []string, err error) {
    top, ns, warns, err := loadSchemaDocument(schemaPath)
    warnings = append(warnings, warns...)
    if err != nil { return "", "", nil, warnings, err }
    if overrideNamespace != "" {
        if err := validateNamespaceStrict(overrideNamespace); err != nil {
            return "", "", nil, warnings, err
        }
        // Replace top-level key with override
        var body any
        for _, v := range top { body = v; break }
        top = map[string]any{overrideNamespace: body}
        ns = overrideNamespace
    }
    // Validate required principals and collect actions
    body := top[ns]
    bmap, ok := body.(map[string]any)
    if !ok { return "", "", nil, warnings, fmt.Errorf("schema namespace %q must map to an object", ns) }
    if err := validateRequiredPrincipals(ns, bmap); err != nil { return "", "", nil, warnings, err }
    acts := collectActionNames(bmap)
    // Encode canonical JSON and enforce size limit
    cj, err := canonicalize(top)
    if err != nil { return "", "", nil, warnings, err }
    return cj, ns, acts, warnings, nil
}

func loadSchemaDocument(schemaPath string) (map[string]any, string, []string, error) {
    raw, err := os.ReadFile(schemaPath)
    if err != nil {
        return nil, "", nil, fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
    }
    var doc any
    switch strings.ToLower(filepath.Ext(schemaPath)) {
    case ".yaml", ".yml":
        if err := yaml.Unmarshal(raw, &doc); err != nil {
            return nil, "", nil, fmt.Errorf("invalid YAML in %s: %w", schemaPath, err)
        }
    case ".json":
        if err := json.Unmarshal(raw, &doc); err != nil {
            return nil, "", nil, fmt.Errorf("invalid JSON in %s: %w", schemaPath, err)
        }
    default:
        return nil, "", nil, fmt.Errorf("unsupported schema extension %q; expected .yaml, .yml, or .json", filepath.Ext(schemaPath))
    }
    top, ok := doc.(map[string]any)
    if !ok {
        return nil, "", nil, fmt.Errorf("schema must be a mapping of namespace → {entityTypes, actions}")
    }
    if len(top) != 1 {
        return nil, "", nil, fmt.Errorf("AVP supports a single namespace per schema; found %d namespaces", len(top))
    }
    var ns string
    for k := range top { ns = k; break }
    // Soft warning when namespace isn't kebab-case; providers may elevate to error
    warns := []string{}
    re := regexp.MustCompile(`^[a-z0-9][a-z0-9-]+$`)
    if !re.MatchString(ns) {
        warns = append(warns, fmt.Sprintf("namespace %q is non-standard; consider simple kebab-case", ns))
    }
    return top, ns, warns, nil
}

func validateNamespaceStrict(ns string) error {
    re := regexp.MustCompile(`^[a-z0-9][a-z0-9-]+$`)
    if !re.MatchString(ns) {
        return fmt.Errorf("invalid namespace %q: must match ^[a-z0-9][a-z0-9-]+$", ns)
    }
    return nil
}

func validateRequiredPrincipals(ns string, bmap map[string]any) error {
    etRaw, ok := bmap["entityTypes"]
    if !ok { return fmt.Errorf("schema namespace %q must define entityTypes", ns) }
    et, ok := etRaw.(map[string]any)
    if !ok { return fmt.Errorf("entityTypes must be an object of entity type definitions") }
    requiredPrincipals := []string{"Tenant", "User", "Role", "GlobalRole", "TenantGrant"}
    missing := []string{}
    for _, r := range requiredPrincipals { if _, ok := et[r]; !ok { missing = append(missing, r) } }
    if len(missing) > 0 { return fmt.Errorf("schema namespace %q missing required principal entity types: %s", ns, strings.Join(missing, ", ")) }
    return nil
}

func collectActionNames(bmap map[string]any) []string {
    acts := []string{}
    if aRaw, ok := bmap["actions"]; ok {
        if amap, ok := aRaw.(map[string]any); ok {
            for name := range amap { acts = append(acts, name) }
        }
    }
    return acts
}

func canonicalize(top map[string]any) (string, error) {
    b, err := json.Marshal(top)
    if err != nil { return "", fmt.Errorf("failed to encode schema as JSON: %w", err) }
    if sz := len(b); sz > 100000 { return "", fmt.Errorf("schema JSON size %d exceeds 100,000 byte limit", sz) }
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
    cfg, err := awsutil.LoadDefault(ctx, region)
    if err != nil {
        return err
    }
    client := vpapi.NewFromConfig(cfg)
    var current string
    getOut, err := client.GetSchema(ctx, &vpapi.GetSchemaInput{PolicyStoreId: &policyStoreId})
    if err == nil && getOut.Schema != nil {
        current = *getOut.Schema
    }
    if util.NormalizeJSON(current) == util.NormalizeJSON(cedarJSON) {
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
    files, err := util.GlobRecursive(dir, "**/*.cedar")
    if err != nil {
        return nil, err
    }
    sort.Strings(files)
    return files, nil
}
