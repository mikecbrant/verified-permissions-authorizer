package dynamo

import "testing"

func TestRoleKeys(t *testing.T) {
	if RoleScopePK("tenant") != "ROLE_SCOPE#tenant" {
		t.Fatalf("RoleScopePK")
	}
	if RoleNameSK("admin") != "ROLE_NAME#admin" {
		t.Fatalf("RoleNameSK")
	}
	pk, sk := RoleIdGSI("rid")
	if pk != "ROLE#rid" || sk != "ROLE#rid" {
		t.Fatalf("RoleIdGSI")
	}
}
