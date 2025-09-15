package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	sharedavp "github.com/mikecbrant/verified-permissions-authorizer/internal/common"
	awsvp "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/verifiedpermissions"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
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
	cedarJSON, ns, actions, warns, err := sharedavp.LoadAndValidateSchema(schemaPath)
	if err != nil {
		return err
	}
	for _, w := range warns {
		ctx.Log.Warn("AVP: "+w, &pulumi.LogArgs{})
	}

	// Action-group enforcement (schema-level, based on action names)
	agMode := strings.ToLower(valueOrDefault(cfg.ActionGroupEnforcement, "error"))
	if violations, err := sharedavp.EnforceActionGroups(actions, agMode); err != nil {
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
		if err := sharedavp.PutSchemaIfChanged(ctx.Context(), id, cedarJSON, regionName); err != nil {
			return "", err
		}
		ctx.Log.Info(fmt.Sprintf("AVP: schema applied for namespace %q (no-op when unchanged)", ns), &pulumi.LogArgs{})
		return "ok", nil
	}).(pulumi.StringOutput)

	// Collect policy files (*.cedar under policyDir)
	files, err := sharedavp.CollectPolicyFiles(policyDir)
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
			Definition:    &awsvp.PolicyDefinitionArgs{Static: &awsvp.PolicyDefinitionStaticArgs{Statement: stmt}},
		}, pulumi.Parent(store))
		if err != nil {
			return fmt.Errorf("failed to create policy for %s: %w", f, err)
		}
		policyIDs = append(policyIDs, pol.ID().ToStringOutput())
	}

	// Optional: canary checks when a file is provided or a default path exists
	// Default: ./authorizer/canaries.yaml (fallback to legacy ./authorize/canaries.yaml for backward compatibility)
	if cfg.CanaryFile == nil || strings.TrimSpace(*cfg.CanaryFile) == "" {
		newDef := "./authorizer/canaries.yaml"
		oldDef := "./authorize/canaries.yaml"
		if _, err := os.Stat(newDef); err == nil {
			cfg.CanaryFile = &newDef
		} else if _, err := os.Stat(oldDef); err == nil {
			cfg.CanaryFile = &oldDef
		}
	}
	if cfg.CanaryFile != nil && strings.TrimSpace(*cfg.CanaryFile) != "" {
		cf := *cfg.CanaryFile
		if !filepath.IsAbs(cf) {
			cwd, _ := os.Getwd()
			cf = filepath.Join(cwd, cf)
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
			if err := sharedavp.RunCombinedCanaries(ctx.Context(), region, id, cf, agMode); err != nil {
				return "", err
			}
			return "ok", nil
		}).(pulumi.StringOutput)
		ctx.Export(fmt.Sprintf("%s-avpCanary", name), canaryStatus)
	}

	return nil
}
