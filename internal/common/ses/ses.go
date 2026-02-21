package ses

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"github.com/mikecbrant/verified-permissions-authorizer/internal/awssdk"
)

var sesIdentityArnRe = regexp.MustCompile(`^arn:(aws|aws-us-gov|aws-cn):ses:([a-z0-9-]+):([0-9]{12}):identity/(.+)$`)

// ValidateSesConfig validates SES email identity configuration for Cognito User Pools developer email sending.
// Returns (accountId, identityName, identityRegion) on success.
func ValidateSesConfig(sourceArn string, from string, replyToEmail *string, userPoolRegion string) (account string, identity string, identityRegion string, err error) {
	// Parse ARN
	m := sesIdentityArnRe.FindStringSubmatch(sourceArn)
	if m == nil {
		return "", "", "", fmt.Errorf("cognito.sesConfig.sourceArn must be an SES identity ARN (…:ses:<region>:<account>:identity/<email-or-domain>)")
	}
	part := m[1]
	identityRegion = m[2]
	account = m[3]
	identity = m[4]

	// Validate 'from'
	addr, err := mail.ParseAddress(from)
	if err != nil || addr.Address == "" || !strings.Contains(addr.Address, "@") {
		return "", "", "", fmt.Errorf("cognito.sesConfig.from must be a valid email address (got %q)", from)
	}
	fromLower := strings.ToLower(addr.Address)

	if strings.Contains(identity, "@") {
		if fromLower != strings.ToLower(identity) {
			return "", "", "", fmt.Errorf("cognito.sesConfig.from must equal the SES email identity %q", identity)
		}
	} else {
		dom := strings.ToLower(identity)
		parts := strings.SplitN(fromLower, "@", 2)
		if len(parts) != 2 {
			return "", "", "", fmt.Errorf("cognito.sesConfig.from must be a valid email address (got %q)", from)
		}
		fromDom := parts[1]
		if fromDom != dom && !strings.HasSuffix(fromDom, "."+dom) {
			return "", "", "", fmt.Errorf("cognito.sesConfig.from (%s) must be an address within domain %q", from, identity)
		}
	}
	if replyToEmail != nil && *replyToEmail != "" {
		if addr, err := mail.ParseAddress(*replyToEmail); err != nil || addr.Address == "" || !strings.Contains(addr.Address, "@") {
			return "", "", "", fmt.Errorf("cognito.sesConfig.replyToEmail must be a valid email address (got %q)", *replyToEmail)
		}
	}

	// Region rules
	inRegionOnly := map[string]struct{}{
		"us-west-1":      {},
		"ap-northeast-3": {},
		"ap-southeast-3": {},
		"eu-west-3":      {},
		"eu-north-1":     {},
		"eu-south-1":     {},
		"sa-east-1":      {},
		"il-central-1":   {},
		"af-south-1":     {},
	}
	if _, ok := inRegionOnly[userPoolRegion]; ok {
		if identityRegion != userPoolRegion {
			return "", "", "", fmt.Errorf("cognito.sesConfig.sourceArn region (%s) must match the Cognito User Pool region (%s) for this Region's in-region-only sending model", identityRegion, userPoolRegion)
		}
		return account, identity, identityRegion, nil
	}

	altFirst := map[string]string{
		"ap-east-1":      "ap-southeast-1",
		"ap-south-2":     "ap-south-1",
		"ap-southeast-4": "ap-southeast-2",
		"ap-southeast-5": "ap-southeast-2",
		"ca-west-1":      "ca-central-1",
		"eu-central-2":   "eu-central-1",
		"eu-south-2":     "eu-west-3",
		"me-central-1":   "eu-central-1",
	}
	if first, ok := altFirst[userPoolRegion]; ok {
		if identityRegion != first {
			return "", "", "", fmt.Errorf("cognito.sesConfig.sourceArn region (%s) must be %s for Cognito region %s", identityRegion, first, userPoolRegion)
		}
		return account, identity, identityRegion, nil
	}

	allowedBC := map[string]struct{}{"us-east-1": {}, "us-west-2": {}, "eu-west-1": {}}
	if identityRegion != userPoolRegion {
		if _, ok := allowedBC[identityRegion]; !ok {
			return "", "", "", fmt.Errorf("cognito.sesConfig.sourceArn region (%s) must either match the Cognito User Pool region (%s) or be one of [us-east-1, us-west-2, eu-west-1] per Cognito+SES cross-region rules", identityRegion, userPoolRegion)
		}
	}
	if awssdk.PartitionForRegion(identityRegion) != awssdk.PartitionForRegion(userPoolRegion) {
		return "", "", "", fmt.Errorf("cognito.sesConfig.sourceArn partition (%s) is incompatible with Cognito region %s", part, userPoolRegion)
	}
	return account, identity, identityRegion, nil
}
