package dynamo

import (
     "context"
     "fmt"

     "github.com/aws/aws-sdk-go-v2/service/dynamodb"
     "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
     awssdk "github.com/mikecbrant/verified-permissions-authorizer/packages/provider/pkg/aws-sdk"
)

// TxPut defines a put with a standard not-exists condition for (PK, SK).
type TxPut struct{
     Item Item
}

// TxCheck defines a condition check (rare in our flows but supported).
type TxCheck struct{
     Key Item
     ConditionExpression string
}

// WriteTransaction composes a TransactWriteItems call using the provided client.
// It applies not-exists conditions for each TxPut and classifies errors.
func WriteTransaction(ctx context.Context, client interface{ TransactWriteItems(context.Context, *dynamodb.TransactWriteItemsInput, ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) }, puts []TxPut, checks []TxCheck, logger awssdk.Logger) error {
     if logger == nil { logger = awssdk.NopLogger{} }
     if len(puts) == 0 && len(checks) == 0 { return nil }
     var actions []types.TransactWriteItem
     // Puts with uniqueness condition on PK and SK
     for i, p := range puts {
         cond := "attribute_not_exists(PK) AND attribute_not_exists(SK)"
         actions = append(actions, types.TransactWriteItem{Put: &types.Put{
             TableName:           nil, // caller must set on client via middleware or pass via options
             Item:                p.Item,
             ConditionExpression: &cond,
         }})
         logger.Debugf("tx.put[%d]: %s", i, previewKeys(p.Item))
     }
     for i, c := range checks {
         actions = append(actions, types.TransactWriteItem{ConditionCheck: &types.ConditionCheck{
             Key:                 c.Key,
             ConditionExpression: &c.ConditionExpression,
         }})
         logger.Debugf("tx.check[%d]: %s", i, previewKeys(c.Key))
     }
     _, err := client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{ TransactItems: actions })
     if err != nil { return classify(err) }
     logger.Infof("tx.ok: %d puts, %d checks", len(puts), len(checks))
     return nil
}

// previewKeys renders a minimal key preview for logs; never include full item content.
func previewKeys(m map[string]types.AttributeValue) string {
     getS := func(k string) string {
         if v, ok := m[k].(*types.AttributeValueMemberS); ok { return v.Value }
         return ""
     }
     return fmt.Sprintf("PK=%q SK=%q GSI1PK=%q GSI1SK=%q", getS("PK"), getS("SK"), getS("GSI1PK"), getS("GSI1SK"))
}
