package dynamo

import "fmt"

// PolicyPK returns the partition key for policy metadata rows.
func PolicyPK() string { return "GLOBAL" }

// PolicyNameSK returns the sort key for policy metadata rows keyed by name.
func PolicyNameSK(name string) string { return fmt.Sprintf("POLICY_NAME#%s", name) }

// PolicyIdGSI returns the (GSI1PK, GSI1SK) pair for looking up a policy by id.
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
