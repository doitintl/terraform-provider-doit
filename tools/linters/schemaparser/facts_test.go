package schemaparser_test

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/schemaparser"
)

func TestSchemaFactsGobEncodingIsDeterministic(t *testing.T) {
	t.Parallel()

	forward := testSchemaFacts(false)
	reverse := testSchemaFacts(true)

	forwardBytes := encodeSchemaFacts(t, forward)
	reverseBytes := encodeSchemaFacts(t, reverse)
	if !bytes.Equal(forwardBytes, reverseBytes) {
		t.Fatal("equivalent schema facts produced different gob encodings")
	}

	var decoded schemaparser.SchemaFacts
	if err := gob.NewDecoder(bytes.NewReader(forwardBytes)).Decode(&decoded); err != nil {
		t.Fatalf("decode schema facts: %v", err)
	}
	if !reflect.DeepEqual(forward, &decoded) {
		t.Errorf("decoded schema facts differ:\n got: %#v\nwant: %#v", &decoded, forward)
	}
}

func TestSchemaFactsStringIsDeterministic(t *testing.T) {
	t.Parallel()

	facts := &schemaparser.SchemaFacts{
		Schemas: map[string]*schemaparser.SchemaInfo{
			"ZebraResourceSchema": nil,
			"AlphaResourceSchema": nil,
		},
	}

	if got, want := facts.String(), "AlphaResourceSchema, ZebraResourceSchema"; got != want {
		t.Errorf("SchemaFacts.String() = %q, want %q", got, want)
	}
}

func encodeSchemaFacts(t *testing.T, facts *schemaparser.SchemaFacts) []byte {
	t.Helper()

	var buffer bytes.Buffer
	if err := gob.NewEncoder(&buffer).Encode(facts); err != nil {
		t.Fatalf("encode schema facts: %v", err)
	}

	return buffer.Bytes()
}

func testSchemaFacts(reverse bool) *schemaparser.SchemaFacts {
	primaryAttrs := map[string]*schemaparser.AttrInfo{}
	nestedAttrs := map[string]*schemaparser.AttrInfo{}

	primaryEntries := []struct {
		name string
		info *schemaparser.AttrInfo
	}{
		{
			name: "string_default",
			info: &schemaparser.AttrInfo{
				Class:         schemaparser.OptionalComputed,
				HasDefault:    true,
				DefaultValue:  "value",
				PlanModifiers: []string{"UseStateForUnknown", "RequiresReplace"},
				NotClearable:  true,
			},
		},
		{
			name: "nested",
			info: &schemaparser.AttrInfo{
				Class:       schemaparser.Optional,
				IsList:      true,
				NestedAttrs: nestedAttrs,
			},
		},
		{
			name: "empty_nested",
			info: &schemaparser.AttrInfo{
				Class:       schemaparser.Optional,
				NestedAttrs: map[string]*schemaparser.AttrInfo{},
			},
		},
		{
			name: "leaf",
			info: &schemaparser.AttrInfo{
				Class: schemaparser.Required,
			},
		},
	}
	nestedEntries := []struct {
		name string
		info *schemaparser.AttrInfo
	}{
		{
			name: "bool_default",
			info: &schemaparser.AttrInfo{
				Class:        schemaparser.OptionalComputed,
				HasDefault:   true,
				DefaultValue: false,
			},
		},
		{
			name: "float_default",
			info: &schemaparser.AttrInfo{
				Class:        schemaparser.OptionalComputed,
				HasDefault:   true,
				DefaultValue: float64(1.5),
			},
		},
		{
			name: "int_default",
			info: &schemaparser.AttrInfo{
				Class:                  schemaparser.OptionalComputed,
				HasDefault:             true,
				DefaultValue:           int64(42),
				RequiresReplaceOnClear: true,
			},
		},
		{
			name: "unknown_default",
			info: &schemaparser.AttrInfo{
				Class:      schemaparser.OptionalComputed,
				HasDefault: true,
			},
		},
	}

	insertAttrEntries(primaryAttrs, primaryEntries, reverse)
	insertAttrEntries(nestedAttrs, nestedEntries, reverse)

	primary := &schemaparser.SchemaInfo{
		FuncName: "PrimaryResourceSchema",
		Attrs:    primaryAttrs,
	}
	secondary := &schemaparser.SchemaInfo{
		FuncName: "SecondaryDataSourceSchema",
		Attrs: map[string]*schemaparser.AttrInfo{
			"id": {
				Class: schemaparser.ComputedOnly,
			},
		},
	}

	schemas := map[string]*schemaparser.SchemaInfo{}
	if reverse {
		schemas[secondary.FuncName] = secondary
		schemas[primary.FuncName] = primary
	} else {
		schemas[primary.FuncName] = primary
		schemas[secondary.FuncName] = secondary
	}

	return &schemaparser.SchemaFacts{Schemas: schemas}
}

func insertAttrEntries(
	destination map[string]*schemaparser.AttrInfo,
	entries []struct {
		name string
		info *schemaparser.AttrInfo
	},
	reverse bool,
) {
	if reverse {
		for i := len(entries) - 1; i >= 0; i-- {
			destination[entries[i].name] = entries[i].info
		}

		return
	}

	for _, entry := range entries {
		destination[entry.name] = entry.info
	}
}
