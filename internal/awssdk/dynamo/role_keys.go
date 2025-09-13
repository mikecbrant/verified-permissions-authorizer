package dynamo

import "fmt"

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
        "PK": StringAttribute(RoleScopePK(scope)),
        "SK": StringAttribute(RoleNameSK(name)),
    }
}

// RoleIdGSIKeys returns a full GSI1PK/GSI1SK pair for roleId lookups.
func RoleIdGSIKeys(roleId string) Item {
    gpk, gsk := RoleIdGSI(roleId)
    return Item{
        "GSI1PK": StringAttribute(gpk),
        "GSI1SK": StringAttribute(gsk),
    }
}
