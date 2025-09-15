package common

import "testing"

func TestEnforceActionGroups(t *testing.T) {
	bad, err := EnforceActionGroups([]string{"GetTenant", "FooBar"}, "warn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bad) != 1 || bad[0] != "FooBar" {
		t.Fatalf("expected FooBar to violate")
	}
	if _, err := EnforceActionGroups([]string{"FooBar"}, "error"); err == nil {
		t.Fatalf("expected error in error mode")
	}
}
