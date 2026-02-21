package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type capturedResource struct {
	Type   string
	Name   string
	Inputs resource.PropertyMap
}

type testMocks struct {
	region    string
	resources []capturedResource
}

func (m *testMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	// Capture the resource
	m.resources = append(m.resources, capturedResource{Type: args.TypeToken, Name: args.Name, Inputs: args.Inputs})
	// Echo inputs as outputs; synthesize an ID
	id := args.Name + "_id"
	out := args.Inputs
	if args.TypeToken == "aws:cognito/userPool:UserPool" {
		out = out.Copy()
		out[resource.PropertyKey("arn")] = resource.NewStringProperty(
			fmt.Sprintf("arn:aws:cognito-idp:%s:123456789012:userpool/%s", m.region, id),
		)
	}
	return id, out, nil
}

func (m *testMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	// Minimal responses for aws:getRegion (token varies by provider versions)
	if strings.Contains(args.Token, "getRegion") {
		return resource.PropertyMap{
			resource.PropertyKey("name"): resource.NewStringProperty(m.region),
		}, nil
	}
	if strings.Contains(args.Token, "getCallerIdentity") {
		return resource.PropertyMap{
			resource.PropertyKey("accountId"): resource.NewStringProperty("123456789012"),
		}, nil
	}
	return resource.PropertyMap{}, nil
}

// Basic smoke test that the component can be instantiated with mocks.
func TestAuthorizerConstructs(t *testing.T) {
	t.Parallel()
	mocks := &testMocks{region: "us-east-1"}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := NewAuthorizerWithPolicyStore(ctx, "test", AuthorizerArgs{})
		return err
	}, pulumi.WithMocks("test", "dev", mocks))
	if err != nil {
		t.Fatalf("construct failed: %v", err)
	}
}

func TestCognito_DefaultNoSesConfig(t *testing.T) {
	t.Parallel()
	mocks := &testMocks{region: "us-east-1"}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := NewAuthorizerWithPolicyStore(ctx, "test", AuthorizerArgs{Cognito: &CognitoConfig{}})
		return err
	}, pulumi.WithMocks("test", "dev", mocks))
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	var up resource.PropertyMap
	for _, r := range mocks.resources {
		if r.Type == "aws:cognito/userPool:UserPool" {
			up = r.Inputs
		}
		if r.Type == "aws:sesv2/emailIdentityPolicy:EmailIdentityPolicy" {
			t.Fatalf("unexpected SES identity policy created when sesConfig is omitted")
		}
	}
	if up == nil {
		t.Fatalf("user pool not created")
	}
	if _, ok := up[resource.PropertyKey("emailConfiguration")]; ok {
		t.Fatalf("emailConfiguration should not be set by default when sesConfig is omitted")
	}
}

func TestCognito_WithSesConfig_ConfiguresEmailAndPolicy(t *testing.T) {
	t.Parallel()
	mocks := &testMocks{region: "us-east-1"}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := NewAuthorizerWithPolicyStore(ctx, "test", AuthorizerArgs{
			Cognito: &CognitoConfig{
				SesConfig: &CognitoSesConfig{
					SourceArn:        "arn:aws:ses:us-east-1:123456789012:identity/example.com",
					From:             "no-reply@example.com",
					ReplyToEmail:     pulumi.StringRef("support@example.com"),
					ConfigurationSet: pulumi.StringRef("prod"),
				},
			},
		})
		return err
	}, pulumi.WithMocks("test", "dev", mocks))
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	var up resource.PropertyMap
	var seenPolicy bool
	var policyBody string
	for _, r := range mocks.resources {
		switch r.Type {
		case "aws:cognito/userPool:UserPool":
			up = r.Inputs
		case "aws:sesv2/emailIdentityPolicy:EmailIdentityPolicy":
			seenPolicy = true
			// Policy is provided as 'policy' input
			if p, ok := r.Inputs[resource.PropertyKey("policy")]; ok {
				if p.IsString() {
					policyBody = p.StringValue()
				}
				if p.IsOutput() {
					out := p.OutputValue()
					if out.Known && out.Element.IsString() {
						policyBody = out.Element.StringValue()
					}
				}
			}
			// Email identity should be the identity name, not the ARN
			if ei, ok := r.Inputs[resource.PropertyKey("emailIdentity")]; ok {
				if ei.StringValue() != "example.com" {
					t.Fatalf("unexpected emailIdentity: %s", ei.StringValue())
				}
			} else {
				t.Fatalf("emailIdentity not set on SES identity policy")
			}
		}
	}
	if up == nil {
		t.Fatalf("user pool not created")
	}
	ec, ok := up[resource.PropertyKey("emailConfiguration")]
	if !ok {
		t.Fatalf("emailConfiguration not set on user pool")
	}
	// emailConfiguration is an object
	ecm := ec.ObjectValue()
	if got := ecm[resource.PropertyKey("emailSendingAccount")].StringValue(); got != "DEVELOPER" {
		t.Fatalf("emailSendingAccount = %q, want DEVELOPER", got)
	}
	if got := ecm[resource.PropertyKey("sourceArn")].StringValue(); got != "arn:aws:ses:us-east-1:123456789012:identity/example.com" {
		t.Fatalf("sourceArn mismatch: %s", got)
	}
	if got := ecm[resource.PropertyKey("fromEmailAddress")].StringValue(); got != "no-reply@example.com" {
		t.Fatalf("fromEmailAddress mismatch: %s", got)
	}
	if got := ecm[resource.PropertyKey("replyToEmailAddress")].StringValue(); got != "support@example.com" {
		t.Fatalf("replyToEmailAddress mismatch: %s", got)
	}
	if got := ecm[resource.PropertyKey("configurationSet")].StringValue(); got != "prod" {
		t.Fatalf("configurationSet mismatch: %s", got)
	}
	if !seenPolicy {
		t.Fatalf("SES identity policy not created")
	}
	// Assert policy JSON mentions the constructed user pool ARN (based on mocked region + synthesized ID)
	if !strings.Contains(policyBody, "cognito-idp:us-east-1:123456789012:userpool/test-userpool_id") {
		t.Fatalf("policy does not reference expected user pool ARN; got: %s", policyBody)
	}
}

func TestCognito_SesConfigValidation(t *testing.T) {
	t.Parallel()
	// Malformed ARN
	{
		mocks := &testMocks{region: "us-east-1"}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := NewAuthorizerWithPolicyStore(ctx, "test", AuthorizerArgs{
				Cognito: &CognitoConfig{SesConfig: &CognitoSesConfig{SourceArn: "arn:aws:iam::123:role/foo", From: "no-reply@example.com"}},
			})
			return err
		}, pulumi.WithMocks("test", "dev", mocks))
		if err == nil || !strings.Contains(err.Error(), "must be an SES identity ARN") {
			t.Fatalf("expected SES ARN validation error, got: %v", err)
		}
	}
	// Domain identity but mismatched from domain
	{
		mocks := &testMocks{region: "us-east-1"}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := NewAuthorizerWithPolicyStore(ctx, "test", AuthorizerArgs{
				Cognito: &CognitoConfig{SesConfig: &CognitoSesConfig{SourceArn: "arn:aws:ses:us-east-1:123456789012:identity/example.com", From: "not-allowed@other.com"}},
			})
			return err
		}, pulumi.WithMocks("test", "dev", mocks))
		if err == nil || !strings.Contains(err.Error(), "must be an address within domain \"example.com\"") {
			t.Fatalf("expected domain validation error, got: %v", err)
		}
	}
	// Email identity but different from
	{
		mocks := &testMocks{region: "us-east-1"}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := NewAuthorizerWithPolicyStore(ctx, "test", AuthorizerArgs{
				Cognito: &CognitoConfig{SesConfig: &CognitoSesConfig{SourceArn: "arn:aws:ses:us-east-1:123456789012:identity/sender@example.com", From: "no-reply@example.com"}},
			})
			return err
		}, pulumi.WithMocks("test", "dev", mocks))
		if err == nil || !strings.Contains(err.Error(), "must equal the SES email identity \"sender@example.com\"") {
			t.Fatalf("expected email identity validation error, got: %v", err)
		}
	}
	// Region in-region-only violation (us-west-1 requires in-region)
	{
		mocks := &testMocks{region: "us-west-1"}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := NewAuthorizerWithPolicyStore(ctx, "test", AuthorizerArgs{
				Cognito: &CognitoConfig{SesConfig: &CognitoSesConfig{SourceArn: "arn:aws:ses:us-east-1:123456789012:identity/example.com", From: "no-reply@example.com"}},
			})
			return err
		}, pulumi.WithMocks("test", "dev", mocks))
		if err == nil || !strings.Contains(err.Error(), "must match the Cognito User Pool region (us-west-1)") {
			t.Fatalf("expected region validation error, got: %v", err)
		}
	}
}

// Validate that the provider schema outputs are grouped under cognito, dynamo, and lambda
// and that legacy flat keys were removed.
func TestSchema_GroupedOutputs(t *testing.T) {
	t.Parallel()
	// Load schema.json relative to this test file
	b, err := os.ReadFile("./schema.json")
	if err != nil {
		t.Fatalf("failed to read schema.json: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(b, &schema); err != nil {
		t.Fatalf("invalid schema.json: %v", err)
	}
	resources, ok := schema["resources"].(map[string]any)
	if !ok {
		t.Fatalf("schema.resources missing or wrong type")
	}
	r, ok := resources["verified-permissions-authorizer:index:AuthorizerWithPolicyStore"].(map[string]any)
	if !ok {
		t.Fatalf("component resource not found in schema")
	}
	props, ok := r["properties"].(map[string]any)
	if !ok {
		t.Fatalf("resource.properties missing or wrong type")
	}
	// Group keys should exist
	if _, ok := props["cognito"]; !ok {
		t.Fatalf("expected grouped output 'cognito'")
	}
	if _, ok := props["dynamo"]; !ok {
		t.Fatalf("expected grouped output 'dynamo'")
	}
	if _, ok := props["lambda"]; !ok {
		t.Fatalf("expected grouped output 'lambda'")
	}
	// Validate required group properties
	lam := props["lambda"].(map[string]any)
	lamProps, _ := lam["properties"].(map[string]any)
	if _, ok := lamProps["authorizerFunctionArn"]; !ok {
		t.Fatalf("expected lambda.authorizerFunctionArn in schema")
	}
	if _, ok := lamProps["roleArn"]; !ok {
		t.Fatalf("expected lambda.roleArn in schema")
	}
	ddb := props["dynamo"].(map[string]any)
	ddbProps, _ := ddb["properties"].(map[string]any)
	if _, ok := ddbProps["authTableArn"]; !ok {
		t.Fatalf("expected dynamo.authTableArn in schema")
	}
	if _, ok := ddbProps["authTableStreamArn"]; !ok {
		t.Fatalf("expected dynamo.authTableStreamArn in schema")
	}
	// Legacy flat keys should be absent at top-level
	for _, k := range []string{
		"AuthTableArn", "AuthTableStreamArn", "authorizerFunctionArn", "roleArn",
		"userPoolId", "userPoolArn", "userPoolDomain", "identityPoolId", "authRoleArn", "unauthRoleArn",
	} {
		if _, exists := props[k]; exists {
			t.Fatalf("unexpected flat output at top-level: %s", k)
		}
	}
}
