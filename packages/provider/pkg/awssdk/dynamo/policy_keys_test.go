package dynamo

import (
    "testing"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestPolicyKeys(t *testing.T) {
    it := PolicyPrimaryKey("pname")
    if v, ok := it["PK"].(*types.AttributeValueMemberS); !ok || v.Value != "GLOBAL" {
        t.Fatalf("bad PK: %#v", it["PK"])
    }
    g := PolicyIdGSIKeys("p1")
    if v, ok := g["GSI1PK"].(*types.AttributeValueMemberS); !ok || v.Value != "POLICY#p1" {
        t.Fatalf("bad GSI1PK: %#v", g["GSI1PK"])
    }
}
