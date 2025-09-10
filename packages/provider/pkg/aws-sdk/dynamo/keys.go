package dynamo

import (
     "fmt"

     "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// KeyValue renders a string AttributeValue.
func KeyValue(s string) types.AttributeValue { return &types.AttributeValueMemberS{Value: s} }

// Item is a shorthand for a DynamoDB item.
type Item = map[string]types.AttributeValue

// Tenant keys
func TenantPK(tenantId string) string { return fmt.Sprintf("TENANT#%s", tenantId) }
func TenantSK(tenantId string) string { return fmt.Sprintf("TENANT#%s", tenantId) }
func TenantNameGSI(name string) (string, string) {
     v := fmt.Sprintf("TENANT_NAME#%s", name)
     return v, v
}
// TenantPrimaryKey returns a full PK/SK pair for a tenant id.
func TenantPrimaryKey(tenantId string) Item {
     return Item{
         "PK": KeyValue(TenantPK(tenantId)),
         "SK": KeyValue(TenantSK(tenantId)),
     }
}
// TenantNameGSIKeys returns a full GSI1PK/GSI1SK pair for a tenant name lookup.
func TenantNameGSIKeys(name string) Item {
     gpk, gsk := TenantNameGSI(name)
     return Item{
         "GSI1PK": KeyValue(gpk),
         "GSI1SK": KeyValue(gsk),
     }
}

// User keys
func UserPK(userId string) string { return fmt.Sprintf("USER#%s", userId) }
func UserSK(userId string) string { return fmt.Sprintf("USER#%s", userId) }
// UserPrimaryKey returns a full PK/SK pair for a user id.
func UserPrimaryKey(userId string) Item {
     return Item{
         "PK": KeyValue(UserPK(userId)),
         "SK": KeyValue(UserSK(userId)),
     }
}

// Uniqueness guards
func UserEmailPK(email string) string { return fmt.Sprintf("USER_EMAIL#%s", email) }
func UserPhonePK(phone string) string { return fmt.Sprintf("USER_PHONE#%s", phone) }
func UserPreferredUsernamePK(u string) string { return fmt.Sprintf("USER_PREFERREDUSERNAME#%s", u) }

// Role keys
func RoleScopePK(scope string) string { return fmt.Sprintf("ROLE_SCOPE#%s", scope) }
func RoleNameSK(name string) string   { return fmt.Sprintf("ROLE_NAME#%s", name) }
func RoleIdGSI(roleId string) (string, string) {
     v := fmt.Sprintf("ROLE#%s", roleId)
     return v, v
}
// RolePrimaryKey returns a full PK/SK pair for a role by scope+name.
func RolePrimaryKey(scope, name string) Item {
     return Item{
         "PK": KeyValue(RoleScopePK(scope)),
         "SK": KeyValue(RoleNameSK(name)),
     }
}
// RoleIdGSIKeys returns a full GSI1PK/GSI1SK pair for roleId lookups.
func RoleIdGSIKeys(roleId string) Item {
     gpk, gsk := RoleIdGSI(roleId)
     return Item{
         "GSI1PK": KeyValue(gpk),
         "GSI1SK": KeyValue(gsk),
     }
}

// TenantGrant keys
func TenantGrantPK(tenantId string) string { return TenantPK(tenantId) }
func TenantGrantSK(userId string) string   { return UserSK(userId) }
func TenantGrantGSI1PK(userId string) string {
     return UserPK(userId)
}
func TenantGrantGSI1SK(tenantId string) string {
     return TenantPK(tenantId)
}
func TenantGrantIdGSI(id string) (string, string) {
     v := fmt.Sprintf("TENANT_GRANT#%s", id)
     return v, v
}
// TenantGrantPrimaryKey returns the (PK, SK) pair for a specific user membership in a tenant.
func TenantGrantPrimaryKey(tenantId, userId string) Item {
     return Item{
         "PK": KeyValue(TenantGrantPK(tenantId)),
         "SK": KeyValue(TenantGrantSK(userId)),
     }
}
// TenantGrantGSI1Keys returns (GSI1PK, GSI1SK) for reverse lookup (user -> tenants).
func TenantGrantGSI1Keys(userId, tenantId string) Item {
     return Item{
         "GSI1PK": KeyValue(TenantGrantGSI1PK(userId)),
         "GSI1SK": KeyValue(TenantGrantGSI1SK(tenantId)),
     }
}
// TenantGrantIdGSIKeys returns (GSI2PK, GSI2SK) for id lookups.
func TenantGrantIdGSIKeys(id string) Item {
     gpk, gsk := TenantGrantIdGSI(id)
     return Item{
         "GSI2PK": KeyValue(gpk),
         "GSI2SK": KeyValue(gsk),
     }
}

// Policy keys
func PolicyPK() string { return "GLOBAL" }
func PolicyNameSK(name string) string {
     return fmt.Sprintf("POLICY_NAME#%s", name)
}
func PolicyIdGSI(id string) (string, string) {
     v := fmt.Sprintf("POLICY#%s", id)
     return v, v
}
// PolicyPrimaryKey returns the (PK, SK) pair for a policy metadata row by name.
func PolicyPrimaryKey(name string) Item {
     return Item{
         "PK": KeyValue(PolicyPK()),
         "SK": KeyValue(PolicyNameSK(name)),
     }
}
// PolicyIdGSIKeys returns (GSI1PK, GSI1SK) for id lookups.
func PolicyIdGSIKeys(id string) Item {
     gpk, gsk := PolicyIdGSI(id)
     return Item{
         "GSI1PK": KeyValue(gpk),
         "GSI1SK": KeyValue(gsk),
     }
}
