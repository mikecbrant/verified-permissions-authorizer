package dynamo

import "testing"

func TestTenantKeys(t *testing.T) {
	if TenantPK("t") != "TENANT#t" {
		t.Fatalf("TenantPK")
	}
	if TenantSK("t") != "TENANT#t" {
		t.Fatalf("TenantSK")
	}
	pk, sk := TenantNameGSI("acme")
	if pk != "TENANT_NAME#acme" || sk != "TENANT_NAME#acme" {
		t.Fatalf("TenantNameGSI")
	}
}
