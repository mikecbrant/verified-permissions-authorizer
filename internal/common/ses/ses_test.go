package ses

import "testing"

func TestValidateSesConfig_EmailIdentityMatch(t *testing.T) {
    acc, ident, region, err := ValidateSesConfig("arn:aws:ses:us-east-1:123456789012:identity/sender@example.com", "sender@example.com", nil, "us-east-1")
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if acc != "123456789012" || ident != "sender@example.com" || region != "us-east-1" { t.Fatalf("unexpected parsed values") }
}

func TestValidateSesConfig_DomainMismatch(t *testing.T) {
    _, _, _, err := ValidateSesConfig("arn:aws:ses:us-east-1:123456789012:identity/example.com", "nope@other.com", nil, "us-east-1")
    if err == nil { t.Fatalf("expected error for domain mismatch") }
}
