package dynamo

import (
    "testing"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestTenantGrantKeys(t *testing.T) {
    it := TenantGrantPrimaryKey("t1", "u1")
    if v, ok := it["PK"].(*types.AttributeValueMemberS); !ok || v.Value != "TENANT#t1" {
        t.Fatalf("bad PK: %#v", it["PK"])
    }
    g := TenantGrantGSI1Keys("u1", "t1")
    if v, ok := g["GSI1PK"].(*types.AttributeValueMemberS); !ok || v.Value != "USER#u1" {
        t.Fatalf("bad GSI1PK: %#v", g["GSI1PK"])
    }
    g2 := TenantGrantIdGSIKeys("tg1")
    if v, ok := g2["GSI2PK"].(*types.AttributeValueMemberS); !ok || v.Value != "TENANT_GRANT#tg1" {
        t.Fatalf("bad GSI2PK: %#v", g2["GSI2PK"])
    }
}
