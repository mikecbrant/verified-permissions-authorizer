package dynamo

import (
     "errors"
     "fmt"

     "github.com/aws/smithy-go"
)

// ConflictError indicates a uniqueness or conditional check failure; callers should not retry blindly.
type ConflictError struct{ Cause error }

func (e *ConflictError) Error() string { return fmt.Sprintf("conflict: %v", e.Cause) }
func (e *ConflictError) Unwrap() error { return e.Cause }

// RetryableError indicates the request may succeed on retry with backoff.
type RetryableError struct{ Cause error }

func (e *RetryableError) Error() string { return fmt.Sprintf("retryable: %v", e.Cause) }
func (e *RetryableError) Unwrap() error { return e.Cause }

// OpError is a generic wrapper for unexpected failures.
type OpError struct{ Cause error }

func (e *OpError) Error() string { return fmt.Sprintf("op error: %v", e.Cause) }
func (e *OpError) Unwrap() error { return e.Cause }

// classify maps smithy errors to our small set.
func classify(err error) error {
     if err == nil {
         return nil
     }
     var api smithy.APIError
     if errors.As(err, &api) {
         switch api.ErrorCode() {
         case "ConditionalCheckFailedException", "TransactionCanceledException":
             return &ConflictError{Cause: err}
         case "ProvisionedThroughputExceededException", "ThrottlingException", "RequestLimitExceeded", "TransactionInProgressException":
             return &RetryableError{Cause: err}
         }
     }
     return &OpError{Cause: err}
}
