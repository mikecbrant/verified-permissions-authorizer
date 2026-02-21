package dynamo

import "fmt"

// TenantPK returns the partition key for a tenant record.
func TenantPK(tenantId string) string { return fmt.Sprintf("TENANT#%s", tenantId) }

// TenantSK returns the sort key for a tenant record.
func TenantSK(tenantId string) string { return fmt.Sprintf("TENANT#%s", tenantId) }

// TenantNameGSI returns the (GSI1PK, GSI1SK) pair for a tenant name uniqueness guard.
func TenantNameGSI(name string) (string, string) {
	v := fmt.Sprintf("TENANT_NAME#%s", name)
	return v, v
}

// TenantPrimaryKey returns a full PK/SK pair for a tenant id.
func TenantPrimaryKey(tenantId string) Item {
	return Item{
		"PK": StringAttribute(TenantPK(tenantId)),
		"SK": StringAttribute(TenantSK(tenantId)),
	}
}

// TenantNameGSIKeys returns a full GSI1PK/GSI1SK pair for a tenant name lookup.
func TenantNameGSIKeys(name string) Item {
	gpk, gsk := TenantNameGSI(name)
	return Item{
		"GSI1PK": StringAttribute(gpk),
		"GSI1SK": StringAttribute(gsk),
	}
}
