package provider

import (
    "fmt"
    "net/mail"
    "regexp"
    "strings"
)

// CognitoSesConfig describes optional SES settings to configure Cognito User Pool email sending.
type CognitoSesConfig struct {
    SourceArn        string  `pulumi:"sourceArn"`
    From             string  `pulumi:"from"`
    ReplyToEmail     *string `pulumi:"replyToEmail,optional"`
    ConfigurationSet *string `pulumi:"configurationSet,optional"`
}

var sesIdentityArnRe = regexp.MustCompile(`^arn:(aws|aws-us-gov|aws-cn):ses:([a-z0-9-]+):([0-9]{12}):identity\/(.+)$`)

// partitionForRegion derives the AWS partition from a region name.
func partitionForRegion(region string) string {
    switch {
    case strings.HasPrefix(region, "cn-"):
        return "aws-cn"
    case strings.HasPrefix(region, "us-gov-"):
        return "aws-us-gov"
    default:
        return "aws"
    }
}

// validateSesConfig performs static validation and domain/email checks. It returns the account id and identity name
// parsed from the sourceArn when valid.
func validateSesConfig(cfg CognitoSesConfig, userPoolRegion string) (account string, identity string, identityRegion string, err error) {
    // Parse ARN
    m := sesIdentityArnRe.FindStringSubmatch(cfg.SourceArn)
    if m == nil {
        return "", "", "", fmt.Errorf("cognito.sesConfig.sourceArn must be an SES identity ARN (…:ses:<region>:<account>:identity/<email-or-domain>)")
    }
    part := m[1]
    identityRegion = m[2]
    account = m[3]
    identity = m[4]

    // Validate 'from' as an email address
    addr, err := mail.ParseAddress(cfg.From)
    if err != nil || addr.Address == "" || !strings.Contains(addr.Address, "@") {
        return "", "", "", fmt.Errorf("cognito.sesConfig.from must be a valid email address (got %q)", cfg.From)
    }
    fromLower := strings.ToLower(addr.Address)

    // Identity-specific rules
    if strings.Contains(identity, "@") {
        // Email identity: from must match exactly
        if fromLower != strings.ToLower(identity) {
            return "", "", "", fmt.Errorf("cognito.sesConfig.from must equal the SES email identity %q", identity)
        }
    } else {
        // Domain identity: from must be within the domain (allow exact domain or subdomains)
        dom := strings.ToLower(identity)
        parts := strings.SplitN(fromLower, "@", 2)
        if len(parts) != 2 {
            return "", "", "", fmt.Errorf("cognito.sesConfig.from must be a valid email address (got %q)", cfg.From)
        }
        fromDom := parts[1]
        if fromDom != dom && !strings.HasSuffix(fromDom, "."+dom) {
            return "", "", "", fmt.Errorf("cognito.sesConfig.from (%s) must be an address within domain %q", cfg.From, identity)
        }
    }

    // Optional validation for replyTo
    if cfg.ReplyToEmail != nil && *cfg.ReplyToEmail != "" {
        if addr, err := mail.ParseAddress(*cfg.ReplyToEmail); err != nil || addr.Address == "" || !strings.Contains(addr.Address, "@") {
            return "", "", "", fmt.Errorf("cognito.sesConfig.replyToEmail must be a valid email address (got %q)", *cfg.ReplyToEmail)
        }
    }

    // Region constraints: enforce common Cognito+SES rules for DEVELOPER sending
    // 1) Regions that require in-region-only SES identities
    inRegionOnly := map[string]struct{}{
        "us-west-1":      {},
        "ap-northeast-3": {}, // Osaka
        "ap-southeast-3": {}, // Jakarta
        "eu-west-3":      {}, // Paris
        "eu-north-1":     {}, // Stockholm
        "eu-south-1":     {}, // Milan
        "sa-east-1":      {}, // Sao Paulo
        "il-central-1":   {}, // Tel Aviv
        "af-south-1":     {}, // Cape Town
    }
    if _, ok := inRegionOnly[userPoolRegion]; ok {
        if identityRegion != userPoolRegion {
            return "", "", "", fmt.Errorf("cognito.sesConfig.sourceArn region (%s) must match the Cognito User Pool region (%s) for this Region's in-region-only sending model", identityRegion, userPoolRegion)
        }
        return account, identity, identityRegion, nil
    }

    // 2) Regions that require an alternate SES Region (first listed). For DEVELOPER, only the first applies.
    altFirst := map[string]string{
        "ap-east-1":      "ap-southeast-1", // Hong Kong -> Singapore
        "ap-south-2":     "ap-south-1",     // Hyderabad -> Mumbai
        "ap-southeast-4": "ap-southeast-2", // Melbourne -> Sydney
        "ap-southeast-5": "ap-southeast-2", // Malaysia -> Sydney
        "ca-west-1":      "ca-central-1",   // Calgary -> Montreal
        "eu-central-2":   "eu-central-1",   // Zurich -> Frankfurt
        "eu-south-2":     "eu-west-3",      // Spain -> Paris
        "me-central-1":   "eu-central-1",   // UAE -> Frankfurt
    }
    if first, ok := altFirst[userPoolRegion]; ok {
        if identityRegion != first {
            return "", "", "", fmt.Errorf("cognito.sesConfig.sourceArn region (%s) must be %s for Cognito region %s", identityRegion, first, userPoolRegion)
        }
        return account, identity, identityRegion, nil
    }

    // 3) Backwards-compatible Regions: allow same-region or one of the three historical SES Regions
    allowedBC := map[string]struct{}{"us-east-1": {}, "us-west-2": {}, "eu-west-1": {}}
    if identityRegion != userPoolRegion {
        if _, ok := allowedBC[identityRegion]; !ok {
            return "", "", "", fmt.Errorf("cognito.sesConfig.sourceArn region (%s) must either match the Cognito User Pool region (%s) or be one of [us-east-1, us-west-2, eu-west-1] per Cognito+SES cross-region rules", identityRegion, userPoolRegion)
        }
    }
    // Partition compatibility
    if partitionForRegion(identityRegion) != partitionForRegion(userPoolRegion) {
        return "", "", "", fmt.Errorf("cognito.sesConfig.sourceArn partition (%s) is incompatible with Cognito region %s", part, userPoolRegion)
    }

    return account, identity, identityRegion, nil
}
