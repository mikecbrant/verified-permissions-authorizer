package dynamo

import (
     "context"
     "errors"
     "testing"

     "github.com/aws/aws-sdk-go-v2/service/dynamodb"
     "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type fakeClient struct{
     in *dynamodb.TransactWriteItemsInput
     err error
}
func (f *fakeClient) TransactWriteItems(ctx context.Context, in *dynamodb.TransactWriteItemsInput, _ ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
     f.in = in
     return &dynamodb.TransactWriteItemsOutput{}, f.err
}

type bufLogger struct{ calls []string }
func (l *bufLogger) Debugf(format string, args ...any) { l.calls = append(l.calls, "debug") }
func (l *bufLogger) Infof(format string, args ...any)  { l.calls = append(l.calls, "info") }
func (l *bufLogger) Warnf(format string, args ...any)  { l.calls = append(l.calls, "warn") }

func TestWriteTransaction_BuildsActions(t *testing.T) {
     c := &fakeClient{}
     l := &bufLogger{}
     item := Item{"PK": KeyValue("A"), "SK": KeyValue("B")}
     if err := WriteTransaction(context.Background(), c, []TxPut{{Item: item}}, nil, l); err != nil {
         t.Fatalf("unexpected err: %v", err)
     }
     if c.in == nil || len(c.in.TransactItems) != 1 || c.in.TransactItems[0].Put == nil {
         t.Fatalf("missing put in transact items: %#v", c.in)
     }
     if cond := c.in.TransactItems[0].Put.ConditionExpression; cond == nil || *cond == "" {
         t.Fatalf("expected not-exists condition on put, got: %v", cond)
     }
     if len(l.calls) == 0 { t.Fatalf("expected logs to be emitted") }
}

// smithy APIError minimal fake
type apiErr struct{ code string }
func (e apiErr) Error() string   { return e.code }
func (e apiErr) ErrorCode() string { return e.code }
func (e apiErr) ErrorFault() string { return "client" }
func (e apiErr) ErrorMessage() string { return e.code }
func (e apiErr) ErrorName() string { return e.code }

func TestClassifyErrors(t *testing.T) {
     tests := []struct{ in error; want string }{
         {apiErr{"ConditionalCheckFailedException"}, "conflict"},
         {apiErr{"TransactionCanceledException"}, "conflict"},
         {apiErr{"ProvisionedThroughputExceededException"}, "retryable"},
         {apiErr{"ThrottlingException"}, "retryable"},
         {apiErr{"RequestLimitExceeded"}, "retryable"},
         {errors.New("boom"), "op error"},
     }
     for _, tt := range tests {
         got := classify(tt.in)
         if got == nil || !errors.Is(got, got) { t.Fatalf("nil or not wrapping: %v", got) }
         if !stringsContains(got.Error(), tt.want) {
             t.Fatalf("classify(%v) => %v; want contains %q", tt.in, got, tt.want)
         }
     }
}

func stringsContains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || (len(sub) > 0 && (func() bool { return contains(s, sub) })())) }
func contains(s, sub string) bool { for i := 0; i+len(sub) <= len(s); i++ { if s[i:i+len(sub)] == sub { return true } } ; return false }

func TestPreviewKeys_Safe(t *testing.T) {
     m := Item{"PK": KeyValue("P"), "SK": KeyValue("S"), "GSI1PK": KeyValue("G1P"), "GSI1SK": KeyValue("G1S"), "secret": &types.AttributeValueMemberS{Value: "x"}}
     s := previewKeys(m)
     if !stringsContains(s, "PK=\"P\"") || !stringsContains(s, "SK=\"S\"") || !stringsContains(s, "GSI1PK=\"G1P\"") {
         t.Fatalf("unexpected preview: %s", s)
     }
     if stringsContains(s, "secret") { t.Fatalf("preview leaked non-key attribute") }
}
