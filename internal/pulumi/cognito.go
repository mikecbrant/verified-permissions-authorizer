package provider

// CognitoConfig captures optional Cognito-related settings for the component.
// Currently only SES email configuration is supported; additional fields can be
// added as the implementation grows.
type CognitoConfig struct {
	// Optional SES settings for configuring Cognito email sending.
	SesConfig *CognitoSesConfig `pulumi:"sesConfig,optional"`
	// Optional set of sign-in aliases to enable on the user pool (e.g., username, email).
	// Present for parity with the overall project schema; not currently used by this component.
	SignInAliases []string `pulumi:"signInAliases,optional"`
}
