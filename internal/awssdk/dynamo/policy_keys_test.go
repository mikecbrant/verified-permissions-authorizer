package dynamo

import "testing"

func TestPolicyKeys(t *testing.T) {
    if PolicyPK() != "GLOBAL" { t.Fatalf("PolicyPK") }
    if PolicyNameSK("n") != "POLICY_NAME#n" { t.Fatalf("PolicyNameSK") }
    pk, sk := PolicyIdGSI("id")
    if pk != "POLICY#id" || sk != "POLICY#id" { t.Fatalf("PolicyIdGSI") }
}
