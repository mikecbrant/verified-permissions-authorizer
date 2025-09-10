package verifiedpermissions

import (
     "context"
     "errors"
     "testing"
)

type fakeAPI struct{ get string; put string; err error }

func (f *fakeAPI) GetSchema(ctx context.Context, in *GetSchemaInput, _ ...func(*Options)) (*GetSchemaOutput, error) {
     if f.err != nil { return nil, f.err }
     if f.get == "" { return &GetSchemaOutput{}, nil }
     return &GetSchemaOutput{ Schema: &f.get }, nil
}
func (f *fakeAPI) PutSchema(ctx context.Context, in *PutSchemaInput, _ ...func(*Options)) (*PutSchemaOutput, error) {
     if f.err != nil { return nil, f.err }
     // capture put body
     if def, ok := in.Definition.(*SchemaDefinitionMemberCedarJson); ok { f.put = def.Value }
     return &PutSchemaOutput{}, nil
}

// Aliases to avoid importing the real package types in test; we compile against our interface.
type (
     GetSchemaInput = struct{ PolicyStoreId *string }
     GetSchemaOutput = struct{ Schema *string }
     PutSchemaInput = struct{ PolicyStoreId *string; Definition any }
     PutSchemaOutput = struct{}
     Options = struct{}
     SchemaDefinitionMemberCedarJson = struct{ Value string }
)

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
