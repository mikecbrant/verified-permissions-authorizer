package verifiedpermissions

import (
     "context"
     "fmt"

     vpapi "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions"
     vptypes "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions/types"
     "github.com/mikecbrant/verified-permissions-authorizer/packages/provider/pkg/jsonutil"
)

// API is the minimal surface used by PutSchemaIfChanged.
type API interface {
     GetSchema(context.Context, *vpapi.GetSchemaInput, ...func(*vpapi.Options)) (*vpapi.GetSchemaOutput, error)
     PutSchema(context.Context, *vpapi.PutSchemaInput, ...func(*vpapi.Options)) (*vpapi.PutSchemaOutput, error)
}

// PutSchemaIfChanged fetches the current schema for the given policy store and
// issues PutSchema only when a semantic change is detected. Comparison uses
// jsonutil.CanonicalizeJSON to avoid differences in whitespace or key order.
//
// Error handling:
// - Any error from GetSchema or PutSchema is returned to the caller for
//   handling at a higher level; this function does not swallow errors.
func PutSchemaIfChanged(ctx context.Context, api API, policyStoreId string, cedarJSON string) error {
     if api == nil { return fmt.Errorf("api is nil") }
     currentOut, err := api.GetSchema(ctx, &vpapi.GetSchemaInput{PolicyStoreId: &policyStoreId})
     if err == nil && currentOut != nil && currentOut.Schema != nil {
         if jsonutil.CanonicalizeJSON(*currentOut.Schema) == jsonutil.CanonicalizeJSON(cedarJSON) {
             return nil
         }
     }
     _, err = api.PutSchema(ctx, &vpapi.PutSchemaInput{
         PolicyStoreId: &policyStoreId,
         Definition:    &vptypes.SchemaDefinitionMemberCedarJson{Value: cedarJSON},
     })
     return err
}
