package testutil

import (
    "context"
    "fmt"
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
type BufferLogger struct{ Calls []string; Entries []string }

func (l *BufferLogger) Debug(msg string, ctx logging.Fields) { l.record("debug", msg, ctx) }
func (l *BufferLogger) Info(msg string, ctx logging.Fields)  { l.record("info", msg, ctx) }
func (l *BufferLogger) Warn(msg string, ctx logging.Fields)  { l.record("warn", msg, ctx) }

func (l *BufferLogger) record(level, msg string, ctx logging.Fields) {
    l.Calls = append(l.Calls, level)
    // simple human-readable capture for assertions; not a JSON serializer
    l.Entries = append(l.Entries, fmt.Sprintf("%s: %s ctx=%v", level, msg, ctx))
}

var _ logging.Logger = (*BufferLogger)(nil)

// Contains reports whether s contains sub; exported for reuse across tests.
func Contains(s, sub string) bool { return strings.Contains(s, sub) }
