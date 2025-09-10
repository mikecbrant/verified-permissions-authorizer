package dynamo

import (
     "context"
     "errors"
     "testing"

     "github.com/mikecbrant/verified-permissions-authorizer/provider/internal/testutil"
)

func TestWriteTransaction_BuildsActions(t *testing.T) {
     c := &testutil.FakeDynamoTxnClient{}
     l := &testutil.BufferLogger{}
     item := Item{"PK": KeyValue("A"), "SK": KeyValue("B")}
     if err := WriteTransaction(context.Background(), c, []TxPut{{Item: item}}, nil, l); err != nil {
         t.Fatalf("unexpected err: %v", err)
     }
     if c.In == nil || len(c.In.TransactItems) != 1 || c.In.TransactItems[0].Put == nil {
         t.Fatalf("missing put in transact items: %#v", c.In)
     }
     if cond := c.In.TransactItems[0].Put.ConditionExpression; cond == nil || *cond == "" {
         t.Fatalf("expected not-exists condition on put, got: %v", cond)
     }
     if len(l.Calls) == 0 { t.Fatalf("expected logs to be emitted") }
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
         if !testutil.Contains(got.Error(), tt.want) {
             t.Fatalf("classify(%v) => %v; want contains %q", tt.in, got, tt.want)
         }
     }
}

func TestWriteTransaction_EmptyInputError(t *testing.T) {
     c := &testutil.FakeDynamoTxnClient{}
     l := &testutil.BufferLogger{}
     if err := WriteTransaction(context.Background(), c, nil, nil, l); err == nil {
         t.Fatalf("expected error on empty input")
     }
}
