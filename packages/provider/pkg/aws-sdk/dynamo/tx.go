package dynamo

import (
     "context"
     "errors"

     "github.com/aws/aws-sdk-go-v2/service/dynamodb"
     "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
     "github.com/mikecbrant/verified-permissions-authorizer/provider/pkg/logging"
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
func WriteTransaction(ctx context.Context, client interface{ TransactWriteItems(context.Context, *dynamodb.TransactWriteItemsInput, ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) }, puts []TxPut, checks []TxCheck, logger logging.Logger) error {
     if logger == nil { logger = logging.NopLogger{} }
     if len(puts) == 0 && len(checks) == 0 { return errors.New("dynamo: WriteTransaction requires at least one put or check") }
     var actions []types.TransactWriteItem
     // Puts with uniqueness condition on PK and SK
     for i, p := range puts {
         cond := "attribute_not_exists(PK) AND attribute_not_exists(SK)"
         actions = append(actions, types.TransactWriteItem{Put: &types.Put{
             TableName:           nil, // caller must set on client via middleware or pass via options
             Item:                p.Item,
             ConditionExpression: &cond,
         }})
         logger.Debugf("tx.put[%d]", i)
     }
     for i, c := range checks {
         actions = append(actions, types.TransactWriteItem{ConditionCheck: &types.ConditionCheck{
             Key:                 c.Key,
             ConditionExpression: &c.ConditionExpression,
         }})
         logger.Debugf("tx.check[%d]", i)
     }
     _, err := client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{ TransactItems: actions })
     if err != nil { return classify(err) }
     logger.Infof("tx.ok: %d puts, %d checks", len(puts), len(checks))
     return nil
}

