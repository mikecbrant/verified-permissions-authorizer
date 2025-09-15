package dynamo

import "testing"

func TestTenantGrantKeys(t *testing.T) {
	if TenantGrantPK("t") != TenantPK("t") {
		t.Fatalf("TenantGrantPK")
	}
	if TenantGrantSK("u") != UserSK("u") {
		t.Fatalf("TenantGrantSK")
	}
	if TenantGrantGSI1PK("u") != UserPK("u") {
		t.Fatalf("TenantGrantGSI1PK")
	}
	if TenantGrantGSI1SK("t") != TenantPK("t") {
		t.Fatalf("TenantGrantGSI1SK")
	}
	pk, sk := TenantGrantIdGSI("id")
	if pk != "TENANT_GRANT#id" || sk != "TENANT_GRANT#id" {
		t.Fatalf("TenantGrantIdGSI")
	}
}
