package dynamo

import "fmt"

// Policy keys (static policy metadata)
func PolicyPK() string                { return "GLOBAL" }
func PolicyNameSK(name string) string { return fmt.Sprintf("POLICY_NAME#%s", name) }
func PolicyIdGSI(id string) (string, string) {
	v := fmt.Sprintf("POLICY#%s", id)
	return v, v
}

// PolicyPrimaryKey returns the (PK, SK) pair for a policy metadata row by name.
func PolicyPrimaryKey(name string) Item {
	return Item{
		"PK": StringAttribute(PolicyPK()),
		"SK": StringAttribute(PolicyNameSK(name)),
	}
}

// PolicyIdGSIKeys returns (GSI1PK, GSI1SK) for id lookups.
func PolicyIdGSIKeys(id string) Item {
	gpk, gsk := PolicyIdGSI(id)
	return Item{
		"GSI1PK": StringAttribute(gpk),
		"GSI1SK": StringAttribute(gsk),
	}
}
