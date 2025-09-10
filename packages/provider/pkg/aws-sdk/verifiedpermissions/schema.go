package verifiedpermissions

import (
     "context"
     "encoding/json"
     "fmt"

     vpapi "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions"
     vptypes "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions/types"
)

// API is the minimal surface used by PutSchemaIfChanged.
type API interface {
     GetSchema(context.Context, *vpapi.GetSchemaInput, ...func(*vpapi.Options)) (*vpapi.GetSchemaOutput, error)
     PutSchema(context.Context, *vpapi.PutSchemaInput, ...func(*vpapi.Options)) (*vpapi.PutSchemaOutput, error)
}

// PutSchemaIfChanged fetches the current schema for the given policy store and
// issues PutSchema only when a semantic change is detected.
//
// Comparison details:
// - The function normalizes both the current and desired Cedar JSON by
//   unmarshaling and re-marshaling ("minification"). This makes comparison
//   resilient to whitespace and key-order differences.
// - If the normalized strings are equal, the call is a no-op and returns nil.
// - Otherwise, PutSchema is invoked with the provided Cedar JSON.
//
// Error handling:
// - Any error from GetSchema or PutSchema is returned to the caller for
//   handling at a higher level; this function does not swallow errors.
func PutSchemaIfChanged(ctx context.Context, api API, policyStoreId string, cedarJSON string) error {
     if api == nil { return fmt.Errorf("api is nil") }
     currentOut, err := api.GetSchema(ctx, &vpapi.GetSchemaInput{PolicyStoreId: &policyStoreId})
     if err == nil && currentOut != nil && currentOut.Schema != nil {
         if minify(*currentOut.Schema) == minify(cedarJSON) {
             return nil
         }
     }
     _, err = api.PutSchema(ctx, &vpapi.PutSchemaInput{
         PolicyStoreId: &policyStoreId,
         Definition:    &vptypes.SchemaDefinitionMemberCedarJson{Value: cedarJSON},
     })
     return err
}

func minify(s string) string {
     var v any
     if err := json.Unmarshal([]byte(s), &v); err != nil { return s }
     b, err := json.Marshal(v); if err != nil { return s }
     return string(b)
}
