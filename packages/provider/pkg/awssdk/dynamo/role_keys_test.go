package dynamo

import (
    "testing"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestRoleKeys(t *testing.T) {
    it := RolePrimaryKey("tenant", "admin")
    if v, ok := it["SK"].(*types.AttributeValueMemberS); !ok || v.Value != "ROLE_NAME#admin" {
        t.Fatalf("bad SK: %#v", it["SK"])
    }
    g := RoleIdGSIKeys("r1")
    if v, ok := g["GSI1PK"].(*types.AttributeValueMemberS); !ok || v.Value != "ROLE#r1" {
        t.Fatalf("bad GSI1PK: %#v", g["GSI1PK"])
    }
}
