package dynamo

import "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

// Item is a shorthand for a DynamoDB item.
type Item = map[string]types.AttributeValue

// StringAttribute renders a string AttributeValue.
func StringAttribute(s string) types.AttributeValue { return &types.AttributeValueMemberS{Value: s} }
