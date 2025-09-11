package dynamo

import (
    "testing"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestTenantKeys(t *testing.T) {
    it := TenantPrimaryKey("t1")
    if v, ok := it["PK"].(*types.AttributeValueMemberS); !ok || v.Value != "TENANT#t1" {
        t.Fatalf("bad PK: %#v", it["PK"])
    }
    if v, ok := it["SK"].(*types.AttributeValueMemberS); !ok || v.Value != "TENANT#t1" {
        t.Fatalf("bad SK: %#v", it["SK"])
    }
    g := TenantNameGSIKeys("Acme")
    if v, ok := g["GSI1PK"].(*types.AttributeValueMemberS); !ok || v.Value != "TENANT_NAME#Acme" {
        t.Fatalf("bad GSI1PK: %#v", g["GSI1PK"])
    }
}
