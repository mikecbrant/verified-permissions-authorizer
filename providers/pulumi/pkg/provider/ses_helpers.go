package provider

import (
    sharedidentity "github.com/mikecbrant/verified-permissions-authorizer/providers/common/identity"
    sharedaws "github.com/mikecbrant/verified-permissions-authorizer/providers/common/awsutil"
)

// CognitoSesConfig describes optional SES settings to configure Cognito User Pool email sending.
type CognitoSesConfig struct {
    SourceArn        string  `pulumi:"sourceArn"`
    From             string  `pulumi:"from"`
    ReplyToEmail     *string `pulumi:"replyToEmail,optional"`
    ConfigurationSet *string `pulumi:"configurationSet,optional"`
}

// partitionForRegion derives the AWS partition from a region name.
func partitionForRegion(region string) string { return sharedaws.PartitionForRegion(region) }

// validateSesConfig performs static validation and domain/email checks. It returns the account id and identity name
// parsed from the sourceArn when valid.
func validateSesConfig(cfg CognitoSesConfig, userPoolRegion string) (account string, identity string, identityRegion string, err error) {
    return sharedidentity.ValidateSesConfig(cfg.SourceArn, cfg.From, cfg.ReplyToEmail, userPoolRegion)
}
