package provider

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "sort"
    "strings"

    vpapi "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions"
    vpapiTypes "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions/types"
    awsvp "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/verifiedpermissions"
    "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
    "gopkg.in/yaml.v3"
)

// VerifiedPermissionsConfig configures where the provider should find the AVP schema (YAML or JSON)
// and Cedar policy files, and how strictly to validate them.
type VerifiedPermissionsConfig struct {
    // Path to schema file (YAML or JSON). YAML is always converted to JSON for validation and upload.
    SchemaFile *string `pulumi:"schemaFile,optional"`
    // Directory containing .cedar policy files (recursively discovered).
    PolicyDir *string `pulumi:"policyDir,optional"`
    // Enforce use of action groups for all policies: off|warn|error (default: error).
    ActionGroupEnforcement *string `pulumi:"actionGroupEnforcement,optional"`
    // Disable installing provider-managed guardrail deny policies (default: false; a warning is emitted when true).
    DisableGuardrails *bool `pulumi:"disableGuardrails,optional"`
    // Optional canary YAML file path. When present (or when default exists), canaries are executed post-deploy.
    CanaryFile *string `pulumi:"canaryFile,optional"`
}

// canonical action group identifiers (PascalCase + Global* variants)
var canonicalActionGroups = []string{
    "BatchCreate", "Create", "BatchDelete", "Delete", "Find", "Get", "BatchUpdate", "Update",
    "GlobalBatchCreate", "GlobalCreate", "GlobalBatchDelete", "GlobalDelete", "GlobalFind", "GlobalGet", "GlobalBatchUpdate", "GlobalUpdate",
}

// applySchemaAndPolicies loads schema/policies from disk, performs validations, applies schema if changed,
// and creates static policies as Pulumi resources bound to the created policy store.
func applySchemaAndPolicies(ctx *pulumi.Context, name string, store *awsvp.PolicyStore, cfg VerifiedPermissionsConfig) error {
    // Resolve inputs (with defaults)
    schemaPath := strings.TrimSpace(valueOrDefault(cfg.SchemaFile, "./authorizer/schema.yaml"))
    policyDir := strings.TrimSpace(valueOrDefault(cfg.PolicyDir, "./authorizer/policies"))
    if !filepath.IsAbs(schemaPath) {
        cwd, _ := os.Getwd()
        schemaPath = filepath.Join(cwd, schemaPath)
    }
    if !filepath.IsAbs(policyDir) {
        cwd, _ := os.Getwd()
        policyDir = filepath.Join(cwd, policyDir)
    }
    if st, err := os.Stat(policyDir); err != nil || !st.IsDir() {
        return fmt.Errorf("verifiedPermissions.policyDir %q not found or not a directory", policyDir)
    }

    // Read and parse schema (YAML or JSON → JSON string)
    cedarJSON, ns, actions, err := loadAndValidateSchema(ctx, schemaPath)
    if err != nil {
        return err
    }

    // Action-group enforcement (schema-level, based on action names)
    agMode := strings.ToLower(valueOrDefault(cfg.ActionGroupEnforcement, "error"))
    if violations, err := enforceActionGroups(actions, agMode); err != nil {
        return err
    } else if len(violations) > 0 && agMode == "warn" {
        ctx.Log.Warn(fmt.Sprintf("AVP: actions not aligned to canonical action groups %v: %s", canonicalActionGroups, strings.Join(violations, ", ")), &pulumi.LogArgs{})
    }

    // Apply schema if changed (best-effort drift detection via GetSchema comparison)
    schemaApplied := pulumi.All(store.ID(), store.Arn).ApplyT(func(args []interface{}) (string, error) {
        id := args[0].(string)
        arn := args[1].(string)
        parts := strings.Split(arn, ":")
        if len(parts) < 4 {
            return "", fmt.Errorf("unexpected policy store ARN: %s", arn)
        }
        regionName := parts[3]
        if err := putSchemaIfChanged(ctx, id, cedarJSON, regionName); err != nil {
            return "", err
        }
        ctx.Log.Info(fmt.Sprintf("AVP: schema applied for namespace %q (no-op when unchanged)", ns), &pulumi.LogArgs{})
        return "ok", nil
    }).(pulumi.StringOutput)

    // Collect policy files (*.cedar under policyDir)
    files, err := globRecursive(policyDir, "**/*.cedar")
    if err != nil {
        return fmt.Errorf("failed to enumerate policies under %s: %w", policyDir, err)
    }
    sort.Strings(files)
    if len(files) == 0 {
        ctx.Log.Warn(fmt.Sprintf("AVP: no .cedar policy files found under %s", policyDir), &pulumi.LogArgs{})
    }

    // Install provider-managed guardrails unless disabled
    disableGuardrails := false
    if cfg.DisableGuardrails != nil {
        disableGuardrails = *cfg.DisableGuardrails
    }
    if disableGuardrails {
        ctx.Log.Warn("Guardrails disabled: provider will not install deny guardrail policies", &pulumi.LogArgs{})
    } else {
        if err := installGuardrails(ctx, name, store, schemaApplied, ns, agMode); err != nil {
            return err
        }
    }

    // Create static policies as child resources (deterministic order)
    policyIDs := []pulumi.StringOutput{}
    for i, f := range files {
        b, err := os.ReadFile(f)
        if err != nil {
            return fmt.Errorf("failed to read policy %s: %w", f, err)
        }
        polName := fmt.Sprintf("%s-pol-%03d", name, i+1)
        // Gate the statement on schema application so policy creation occurs after PutSchema completes.
        stmt := pulumi.All(schemaApplied).ApplyT(func(_ []interface{}) string { return string(b) }).(pulumi.StringOutput)
        pol, err := awsvp.NewPolicy(ctx, polName, &awsvp.PolicyArgs{
            PolicyStoreId: store.ID(),
            Definition: &awsvp.PolicyDefinitionArgs{Static: &awsvp.PolicyDefinitionStaticArgs{Statement: stmt}},
        }, pulumi.Parent(store))
        if err != nil {
            return fmt.Errorf("failed to create policy for %s: %w", f, err)
        }
        policyIDs = append(policyIDs, pol.ID().ToStringOutput())
    }

    // Optional: canary checks when a file is provided or a default path exists
    // Default: ./authorize/canaries.yaml
    if cfg.CanaryFile == nil || strings.TrimSpace(*cfg.CanaryFile) == "" {
        def := "./authorize/canaries.yaml"
        if _, err := os.Stat(def); err == nil {
            cfg.CanaryFile = &def
        }
    }
    if cfg.CanaryFile != nil && strings.TrimSpace(*cfg.CanaryFile) != "" {
        cf := *cfg.CanaryFile
        if !filepath.IsAbs(cf) {
            cwd, _ := os.Getwd()
            cf = filepath.Join(cwd, cf)
        }
        canaryDeps := append([]pulumi.Output{schemaApplied}, toOutputs(policyIDs)...)
        depsAny := outputsToInterfaces(canaryDeps)
        canaryStatus := pulumi.All(depsAny...).ApplyT(func(_ []interface{}) (string, error) {
            if err := runCombinedCanaries(ctx, store, cf, agMode); err != nil {
                return "", err
            }
            return "ok", nil
        }).(pulumi.StringOutput)
        ctx.Export(fmt.Sprintf("%s-avpCanary", name), canaryStatus)
    }

    return nil
}

// loadAndValidateSchema parses YAML/JSON schema, enforces single namespace and required entities.
// Returns cedar JSON string, namespace name, and the set of action names.
func loadAndValidateSchema(ctx *pulumi.Context, schemaPath string) (string, string, []string, error) {
    raw, err := os.ReadFile(schemaPath)
    if err != nil {
        return "", "", nil, fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
    }
    var doc any
    switch strings.ToLower(filepath.Ext(schemaPath)) {
    case ".yaml", ".yml":
        if err := yaml.Unmarshal(raw, &doc); err != nil {
            return "", "", nil, fmt.Errorf("invalid YAML in %s: %w", schemaPath, err)
        }
    case ".json":
        if err := json.Unmarshal(raw, &doc); err != nil {
            return "", "", nil, fmt.Errorf("invalid JSON in %s: %w", schemaPath, err)
        }
    default:
        return "", "", nil, fmt.Errorf("unsupported schema extension %q; expected .yaml, .yml, or .json", filepath.Ext(schemaPath))
    }
    // Expect top-level object: { "<namespace>": { entityTypes: {...}, actions: {...} } }
    top, ok := doc.(map[string]any)
    if !ok {
        return "", "", nil, fmt.Errorf("schema must be a mapping of namespace → {entityTypes, actions}")
    }
    if len(top) != 1 {
        return "", "", nil, fmt.Errorf("AVP supports a single namespace per schema; found %d namespaces", len(top))
    }
    var ns string
    var body any
    for k, v := range top {
        ns = k
        body = v
        break
    }
    // Namespace uniqueness target (warn-only): encourage simple kebab-case
    re := regexp.MustCompile(`^[a-z0-9][a-z0-9-]+$`)
    if !re.MatchString(ns) {
        ctx.Log.Warn(fmt.Sprintf("AVP: namespace %q is non-standard; consider a simple, kebab-case identifier (warn-only)", ns), &pulumi.LogArgs{})
    }

    // Required entity presence checks
    bmap, ok := body.(map[string]any)
    if !ok {
        return "", "", nil, fmt.Errorf("schema namespace %q must map to an object", ns)
    }
    etRaw, ok := bmap["entityTypes"]
    if !ok {
        return "", "", nil, fmt.Errorf("schema namespace %q must define entityTypes", ns)
    }
    et, ok := etRaw.(map[string]any)
    if !ok {
        return "", "", nil, fmt.Errorf("entityTypes must be an object of entity type definitions")
    }
    requiredPrincipals := []string{"Tenant", "User", "Role", "GlobalRole", "TenantGrant"}
    missing := []string{}
    for _, r := range requiredPrincipals {
        if _, ok := et[r]; !ok {
            missing = append(missing, r)
        }
    }
    if len(missing) > 0 {
        return "", "", nil, fmt.Errorf("schema namespace %q missing required principal entity types: %s", ns, strings.Join(missing, ", "))
    }
    // Hierarchy expectation: Tenant supports homogeneous tree → memberOfTypes includes Tenant
    if def, ok := et["Tenant"].(map[string]any); ok {
        mot, _ := def["memberOfTypes"].([]any)
        hasSelf := false
        for _, v := range mot {
            if s, ok := v.(string); ok && s == "Tenant" {
                hasSelf = true
                break
            }
        }
        if !hasSelf {
            ctx.Log.Warn("entity Tenant should include itself in memberOfTypes to enable hierarchical nesting", &pulumi.LogArgs{})
        }
    }
    // Collect action names for action-group enforcement
    acts := []string{}
    if aRaw, ok := bmap["actions"]; ok {
        if amap, ok := aRaw.(map[string]any); ok {
            for name := range amap {
                acts = append(acts, name)
            }
        }
    }
    // Re-encode to canonical JSON (minified) for PutSchema
    b, err := json.Marshal(top)
    if err != nil {
        return "", "", nil, fmt.Errorf("failed to encode schema as JSON: %w", err)
    }
    // Size validation per AVP limit: 100,000 bytes
    if sz := len(b); sz > 100000 {
        return "", "", nil, fmt.Errorf("schema JSON size %d exceeds 100,000 byte limit", sz)
    } else if sz >= 95000 {
        ctx.Log.Warn(fmt.Sprintf("schema JSON size %d is >= 95%% of 100,000 byte limit", sz), &pulumi.LogArgs{})
    }
    return string(b), ns, acts, nil
}

// enforceActionGroups checks that action names map cleanly to canonical groups via exact, case-sensitive prefixes.
func enforceActionGroups(actions []string, mode string) ([]string, error) {
    if strings.EqualFold(mode, "off") {
        return nil, nil
    }
    bad := []string{}
    for _, a := range actions {
        ok := false
        for _, g := range canonicalActionGroups {
            if strings.HasPrefix(a, g) { // exact, case-sensitive prefix
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
    // warn
    return bad, nil
}

// putSchemaIfChanged retrieves existing schema and applies only when content differs.
func putSchemaIfChanged(ctx *pulumi.Context, policyStoreId string, cedarJSON string, region string) error {
    cfg, err := loadAwsConfig(ctx.Context(), region)
    if err != nil {
        return err
    }
    client := vpapi.NewFromConfig(cfg)

    // Fetch current schema; treat NotFound as empty
    var current string
    pulumiCtx := ctx.Context()
    getOut, err := client.GetSchema(pulumiCtx, &vpapi.GetSchemaInput{PolicyStoreId: &policyStoreId})
    if err == nil && getOut.Schema != nil {
        current = *getOut.Schema
    }
    if normalizeJson(current) == normalizeJson(cedarJSON) {
        ctx.Log.Info("AVP: schema unchanged; skipping PutSchema", &pulumi.LogArgs{})
        return nil
    }
    // Apply
    _, err = client.PutSchema(pulumiCtx, &vpapi.PutSchemaInput{
        PolicyStoreId: &policyStoreId,
        Definition:    &vpapiTypes.SchemaDefinitionMemberCedarJson{Value: cedarJSON},
    })
    if err != nil {
        return fmt.Errorf("failed to put schema: %w", err)
    }
    return nil
}
