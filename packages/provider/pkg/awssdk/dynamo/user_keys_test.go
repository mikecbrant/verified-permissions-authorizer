package dynamo

import (
    "testing"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestUserKeys(t *testing.T) {
    it := UserPrimaryKey("u1")
    if v, ok := it["PK"].(*types.AttributeValueMemberS); !ok || v.Value != "USER#u1" {
        t.Fatalf("bad PK: %#v", it["PK"])
    }
    if UserEmailPK("a@b") != "USER_EMAIL#a@b" {
        t.Fatalf("bad UserEmailPK")
    }
}
