package dynamo

import "fmt"

// TenantGrantPK returns the partition key for a tenant membership record.
func TenantGrantPK(tenantId string) string { return TenantPK(tenantId) }

// TenantGrantSK returns the sort key for a tenant membership record.
func TenantGrantSK(userId string) string { return UserSK(userId) }

// TenantGrantGSI1PK returns the GSI1 partition key for a reverse lookup (user -> tenants).
func TenantGrantGSI1PK(userId string) string { return UserPK(userId) }

// TenantGrantGSI1SK returns the GSI1 sort key for a reverse lookup (user -> tenants).
func TenantGrantGSI1SK(tenantId string) string { return TenantPK(tenantId) }

// TenantGrantIdGSI returns the (GSI2PK, GSI2SK) pair for looking up a tenant grant by id.
func TenantGrantIdGSI(id string) (string, string) {
	v := fmt.Sprintf("TENANT_GRANT#%s", id)
	return v, v
}

// TenantGrantPrimaryKey returns the (PK, SK) pair for a specific user membership in a tenant.
func TenantGrantPrimaryKey(tenantId, userId string) Item {
	return Item{
		"PK": StringAttribute(TenantGrantPK(tenantId)),
		"SK": StringAttribute(TenantGrantSK(userId)),
	}
}

// TenantGrantGSI1Keys returns (GSI1PK, GSI1SK) for reverse lookup (user -> tenants).
func TenantGrantGSI1Keys(userId, tenantId string) Item {
	return Item{
		"GSI1PK": StringAttribute(TenantGrantGSI1PK(userId)),
		"GSI1SK": StringAttribute(TenantGrantGSI1SK(tenantId)),
	}
}

// TenantGrantIdGSIKeys returns (GSI2PK, GSI2SK) for id lookups.
func TenantGrantIdGSIKeys(id string) Item {
	gpk, gsk := TenantGrantIdGSI(id)
	return Item{
		"GSI2PK": StringAttribute(gpk),
		"GSI2SK": StringAttribute(gsk),
	}
}
