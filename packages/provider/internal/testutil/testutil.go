package testutil

import (
    "context"
    "strings"

    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/mikecbrant/verified-permissions-authorizer/provider/pkg/logging"
)

// FakeDynamoTxnClient is a minimal fake for TransactWriteItems used in tests.
type FakeDynamoTxnClient struct{
    In  *dynamodb.TransactWriteItemsInput
    Err error
}

func (f *FakeDynamoTxnClient) TransactWriteItems(ctx context.Context, in *dynamodb.TransactWriteItemsInput, _ ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
    f.In = in
    return &dynamodb.TransactWriteItemsOutput{}, f.Err
}

// BufferLogger is a buffer-backed logger that records calls for assertions.
type BufferLogger struct{ Calls []string }

func (l *BufferLogger) Debugf(string, ...any) { l.Calls = append(l.Calls, "debug") }
func (l *BufferLogger) Infof(string, ...any)  { l.Calls = append(l.Calls, "info") }
func (l *BufferLogger) Warnf(string, ...any)  { l.Calls = append(l.Calls, "warn") }

var _ logging.Logger = (*BufferLogger)(nil)

// Contains reports whether s contains sub; exported for reuse across tests.
func Contains(s, sub string) bool { return strings.Contains(s, sub) }
