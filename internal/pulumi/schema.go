package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	awsvp "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/verifiedpermissions"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	sharedavp "github.com/mikecbrant/verified-permissions-authorizer/internal/common"
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
	schemaPath, policyDir, err := resolveSchemaAndPolicyPaths(cfg)
	if err != nil {
		return err
	}

	// Read and parse schema (YAML or JSON → JSON string)
	cedarJSON, ns, actions, warns, err := sharedavp.LoadAndValidateSchema(schemaPath)
	if err != nil {
		return err
	}
	if err := warnAll(ctx, prefixAll("AVP: ", warns)); err != nil {
		return err
	}

	// Action-group enforcement (schema-level, based on action names)
	agMode, err := enforceActionGroups(ctx, actions, cfg)
	if err != nil {
		return err
	}

	// Apply schema if changed (best-effort drift detection via GetSchema comparison)
	schemaApplied := applySchemaIfChanged(ctx, store, cedarJSON, ns)

	// Collect policy files (*.cedar under policyDir)
	files, err := collectPolicyFiles(ctx, policyDir)
	if err != nil {
		return err
	}

	// Install provider-managed guardrails unless disabled
	if err := maybeInstallGuardrails(ctx, name, store, schemaApplied, ns, agMode, cfg); err != nil {
		return err
	}

	// Create static policies as child resources (deterministic order)
	policyIDs, err := createStaticPolicies(ctx, name, store, schemaApplied, files)
	if err != nil {
		return err
	}

	// Optional: canary checks when a file is provided or a default path exists
	// Default: ./authorizer/canaries.yaml (fallback to legacy ./authorize/canaries.yaml for backward compatibility)
	ctx.Export(fmt.Sprintf("%s-policyStoreId", name), store.ID())
	ctx.Export(fmt.Sprintf("%s-policyStoreArn", name), store.Arn)
	ctx.Export(fmt.Sprintf("%s-avpNamespace", name), pulumi.String(ns))
	return maybeExportCanaryStatus(ctx, name, store, schemaApplied, policyIDs, agMode, cfg)
}

func resolveSchemaAndPolicyPaths(cfg VerifiedPermissionsConfig) (schemaPath string, policyDir string, err error) {
	schemaPath = strings.TrimSpace(valueOrDefault(cfg.SchemaFile, "./authorizer/schema.yaml"))
	policyDir = strings.TrimSpace(valueOrDefault(cfg.PolicyDir, "./authorizer/policies"))
	if !filepath.IsAbs(schemaPath) {
		cwd, _ := os.Getwd()
		schemaPath = filepath.Join(cwd, schemaPath)
	}
	if !filepath.IsAbs(policyDir) {
		cwd, _ := os.Getwd()
		policyDir = filepath.Join(cwd, policyDir)
	}
	if st, err := os.Stat(policyDir); err != nil || !st.IsDir() {
		return "", "", fmt.Errorf("verifiedPermissions.policyDir %q not found or not a directory", policyDir)
	}
	return schemaPath, policyDir, nil
}

func prefixAll(prefix string, ins []string) []string {
	outs := make([]string, 0, len(ins))
	for _, in := range ins {
		outs = append(outs, prefix+in)
	}
	return outs
}

func warnAll(ctx *pulumi.Context, msgs []string) error {
	for _, msg := range msgs {
		if err := ctx.Log.Warn(msg, &pulumi.LogArgs{}); err != nil {
			return err
		}
	}
	return nil
}

func enforceActionGroups(ctx *pulumi.Context, actions []string, cfg VerifiedPermissionsConfig) (string, error) {
	agMode := strings.ToLower(valueOrDefault(cfg.ActionGroupEnforcement, "error"))
	violations, err := sharedavp.EnforceActionGroups(actions, agMode)
	if err != nil {
		return "", err
	}
	if len(violations) > 0 && agMode == "warn" {
		msg := fmt.Sprintf(
			"AVP: actions not aligned to canonical action groups %v: %s",
			canonicalActionGroups,
			strings.Join(violations, ", "),
		)
		if err := ctx.Log.Warn(msg, &pulumi.LogArgs{}); err != nil {
			return "", err
		}
	}
	return agMode, nil
}

func applySchemaIfChanged(ctx *pulumi.Context, store *awsvp.PolicyStore, cedarJSON string, ns string) pulumi.StringOutput {
	return pulumi.All(store.ID(), store.Arn).ApplyT(func(args []interface{}) (string, error) {
		id := args[0].(string)
		arn := args[1].(string)
		parts := strings.Split(arn, ":")
		if len(parts) < 4 {
			return "", fmt.Errorf("unexpected policy store ARN: %s", arn)
		}
		regionName := parts[3]
		if err := sharedavp.PutSchemaIfChanged(ctx.Context(), id, cedarJSON, regionName); err != nil {
			return "", err
		}
		if err := ctx.Log.Info(fmt.Sprintf("AVP: schema applied for namespace %q (no-op when unchanged)", ns), &pulumi.LogArgs{}); err != nil {
			return "", err
		}
		return "ok", nil
	}).(pulumi.StringOutput)
}

func collectPolicyFiles(ctx *pulumi.Context, policyDir string) ([]string, error) {
	files, err := sharedavp.CollectPolicyFiles(policyDir)
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate policies under %s: %w", policyDir, err)
	}
	sort.Strings(files)
	if len(files) == 0 {
		if err := ctx.Log.Warn(fmt.Sprintf("AVP: no .cedar policy files found under %s", policyDir), &pulumi.LogArgs{}); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func maybeInstallGuardrails(ctx *pulumi.Context, name string, store *awsvp.PolicyStore, schemaApplied pulumi.StringOutput, ns string, agMode string, cfg VerifiedPermissionsConfig) error {
	disableGuardrails := false
	if cfg.DisableGuardrails != nil {
		disableGuardrails = *cfg.DisableGuardrails
	}
	if disableGuardrails {
		return ctx.Log.Warn("Guardrails disabled: provider will not install deny guardrail policies", &pulumi.LogArgs{})
	}
	return installGuardrails(ctx, name, store, schemaApplied, ns, agMode)
}

func createStaticPolicies(ctx *pulumi.Context, name string, store *awsvp.PolicyStore, schemaApplied pulumi.StringOutput, files []string) ([]pulumi.StringOutput, error) {
	policyIDs := []pulumi.StringOutput{}
	for i, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("failed to read policy %s: %w", f, err)
		}
		polName := fmt.Sprintf("%s-pol-%03d", name, i+1)
		// Gate the statement on schema application so policy creation occurs after PutSchema completes.
		stmt := pulumi.All(schemaApplied).ApplyT(func(_ []interface{}) string { return string(b) }).(pulumi.StringOutput)
		pol, err := awsvp.NewPolicy(ctx, polName, &awsvp.PolicyArgs{
			PolicyStoreId: store.ID(),
			Definition:    &awsvp.PolicyDefinitionArgs{Static: &awsvp.PolicyDefinitionStaticArgs{Statement: stmt}},
		}, pulumi.Parent(store))
		if err != nil {
			return nil, fmt.Errorf("failed to create policy for %s: %w", f, err)
		}
		policyIDs = append(policyIDs, pol.ID().ToStringOutput())
	}
	return policyIDs, nil
}

func maybeExportCanaryStatus(ctx *pulumi.Context, name string, store *awsvp.PolicyStore, schemaApplied pulumi.StringOutput, policyIDs []pulumi.StringOutput, agMode string, cfg VerifiedPermissionsConfig) error {
	canaryPath, ok := resolveCanaryFile(cfg)
	if !ok {
		return nil
	}
	if !filepath.IsAbs(canaryPath) {
		cwd, _ := os.Getwd()
		canaryPath = filepath.Join(cwd, canaryPath)
	}

	canaryDeps := append([]pulumi.Output{schemaApplied, store.ID().ToStringOutput(), store.Arn}, toOutputs(policyIDs)...)
	depsAny := outputsToInterfaces(canaryDeps)
	canaryStatus := pulumi.All(depsAny...).ApplyT(func(args []interface{}) (string, error) {
		id, ok1 := args[1].(string)
		arn, ok2 := args[2].(string)
		if !ok1 || id == "" || !ok2 || arn == "" {
			return "", fmt.Errorf("failed to resolve policy store id/arn for canary execution")
		}
		parts := strings.Split(arn, ":")
		if len(parts) < 4 {
			return "", fmt.Errorf("unexpected policy store ARN: %s", arn)
		}
		region := parts[3]
		if err := sharedavp.RunCombinedCanaries(ctx.Context(), region, id, canaryPath, agMode); err != nil {
			return "", err
		}
		return "ok", nil
	}).(pulumi.StringOutput)
	ctx.Export(fmt.Sprintf("%s-avpCanary", name), canaryStatus)
	return nil
}

func resolveCanaryFile(cfg VerifiedPermissionsConfig) (string, bool) {
	if cfg.CanaryFile != nil && strings.TrimSpace(*cfg.CanaryFile) != "" {
		return *cfg.CanaryFile, true
	}

	newDef := "./authorizer/canaries.yaml"
	oldDef := "./authorize/canaries.yaml"
	if _, err := os.Stat(newDef); err == nil {
		return newDef, true
	}
	if _, err := os.Stat(oldDef); err == nil {
		return oldDef, true
	}
	return "", false
}
