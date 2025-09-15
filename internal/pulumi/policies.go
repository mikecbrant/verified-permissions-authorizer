package provider

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"

	awsvp "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/verifiedpermissions"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

//go:embed assets/guardrails/*.cedar
var guardrailFS embed.FS

// installGuardrails installs provider-managed guardrail policies as child resources.
// - Base guardrails are always applied (unless DisableGuardrails=true).
// - Action-enforcement guardrail is applied when actionGroupEnforcement != "off".
func installGuardrails(
	ctx *pulumi.Context,
	name string,
	store *awsvp.PolicyStore,
	after pulumi.StringOutput,
	namespace string,
	agMode string,
) error {
	// Load base guardrails
	files := []string{"assets/guardrails/base.cedar"}
	if !strings.EqualFold(agMode, "off") {
		files = append(files, "assets/guardrails/action-enforcement.cedar")
	}
	for _, f := range files {
		b, err := guardrailFS.ReadFile(f)
		if err != nil {
			return fmt.Errorf("failed to read embedded guardrail %s: %w", f, err)
		}
		// Simple namespace interpolation placeholder: ${NAMESPACE}
		text := strings.ReplaceAll(string(b), "${NAMESPACE}", namespace)
		base := filepath.Base(f)
		resName := fmt.Sprintf("%s-%s", name, strings.TrimSuffix(base, filepath.Ext(base)))
		stmt := pulumi.All(after).ApplyT(func(_ []interface{}) string { return text }).(pulumi.StringOutput)
		if _, err := awsvp.NewPolicy(ctx, resName, &awsvp.PolicyArgs{
			PolicyStoreId: store.ID(),
			Definition:    &awsvp.PolicyDefinitionArgs{Static: &awsvp.PolicyDefinitionStaticArgs{Statement: stmt}},
		}, pulumi.Parent(store)); err != nil {
			return fmt.Errorf("failed to create guardrail policy %s: %w", base, err)
		}
	}
	return nil
}
