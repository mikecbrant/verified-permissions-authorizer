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

// User keys
func UserPK(userId string) string { return fmt.Sprintf("USER#%s", userId) }
func UserSK(userId string) string { return fmt.Sprintf("USER#%s", userId) }

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

// Policy keys
func PolicyPK() string { return "GLOBAL" }
func PolicyNameSK(name string) string {
     return fmt.Sprintf("POLICY_NAME#%s", name)
}
func PolicyIdGSI(id string) (string, string) {
     v := fmt.Sprintf("POLICY#%s", id)
     return v, v
}
