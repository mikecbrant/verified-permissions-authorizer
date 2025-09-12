package verifiedpermissions

import (
     "context"
     "errors"
     "testing"

     vpapi "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions"
     vptypes "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions/types"
)

// fakeAPI implements the real vpapi surface required by the local API interface.
type fakeAPI struct{ get string; put string; err error }

func (f *fakeAPI) GetSchema(ctx context.Context, in *vpapi.GetSchemaInput, _ ...func(*vpapi.Options)) (*vpapi.GetSchemaOutput, error) {
     if f.err != nil { return nil, f.err }
     if f.get == "" { return &vpapi.GetSchemaOutput{}, nil }
     return &vpapi.GetSchemaOutput{ Schema: &f.get }, nil
}
func (f *fakeAPI) PutSchema(ctx context.Context, in *vpapi.PutSchemaInput, _ ...func(*vpapi.Options)) (*vpapi.PutSchemaOutput, error) {
     if f.err != nil { return nil, f.err }
     // capture put body
     if def, ok := in.Definition.(*vptypes.SchemaDefinitionMemberCedarJson); ok {
         f.put = def.Value
     }
     return &vpapi.PutSchemaOutput{}, nil
}

func TestPutSchemaIfChanged_ShortCircuits(t *testing.T) {
     api := &fakeAPI{ get: `{"a":1, "b":2}` }
     if err := PutSchemaIfChanged(context.Background(), api, "ps-1", `{"b":2,"a":1}`); err != nil {
         t.Fatalf("unexpected err: %v", err)
     }
     if api.put != "" { t.Fatalf("expected no PutSchema when unchanged; got %q", api.put) }
}

func TestPutSchemaIfChanged_PutsOnChange(t *testing.T) {
     api := &fakeAPI{ get: `{"a":1}` }
     if err := PutSchemaIfChanged(context.Background(), api, "ps-1", `{"a":2}`); err != nil {
         t.Fatalf("unexpected err: %v", err)
     }
     if api.put == "" { t.Fatalf("expected PutSchema to be called") }
}

func TestPutSchemaIfChanged_PropagatesError(t *testing.T) {
     boom := errors.New("boom")
     api := &fakeAPI{ err: boom }
     if err := PutSchemaIfChanged(context.Background(), api, "ps-1", `{}`); err == nil {
         t.Fatalf("expected error")
     }
}
