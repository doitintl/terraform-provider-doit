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

// TestSchemaFactsGobRoundTripCoversEveryField fills every field of SchemaInfo
// and AttrInfo with a non-zero value and requires each to survive a gob round
// trip. A field added to the structs but not to the wire format decodes as its
// zero value here and fails the test, naming the field.
func TestSchemaFactsGobRoundTripCoversEveryField(t *testing.T) {
	t.Parallel()

	schemaInfo := &schemaparser.SchemaInfo{}
	fillNonZero(t, reflect.ValueOf(schemaInfo).Elem(), 0)

	facts := &schemaparser.SchemaFacts{
		Schemas: map[string]*schemaparser.SchemaInfo{"FilledSchema": schemaInfo},
	}

	var decoded schemaparser.SchemaFacts
	if err := gob.NewDecoder(bytes.NewReader(encodeSchemaFacts(t, facts))).Decode(&decoded); err != nil {
		t.Fatalf("decode schema facts: %v", err)
	}

	got, ok := decoded.Schemas["FilledSchema"]
	if !ok {
		t.Fatal("FilledSchema missing after round trip")
	}
	requireNoZeroFields(t, reflect.ValueOf(got).Elem(), "SchemaInfo")

	if !reflect.DeepEqual(schemaInfo, got) {
		t.Errorf("round trip changed schema info:\n got: %#v\nwant: %#v", got, schemaInfo)
	}
}

// maxFillDepth bounds the recursion through NestedAttrs. Two levels cover the
// top-level attribute and one nested level, so nested decoding is exercised.
const maxFillDepth = 2

// fillNonZero sets every field reachable from v to a non-zero value. It fails
// the test on a kind it does not know how to fill, so a new field of a new
// type forces this helper to be extended instead of being skipped.
func fillNonZero(t *testing.T, v reflect.Value, depth int) {
	t.Helper()

	switch v.Kind() {
	case reflect.Bool:
		v.SetBool(true)
	case reflect.String:
		v.SetString("x")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Interface:
		// DefaultValue is an `any` holding a static default.
		v.Set(reflect.ValueOf("x"))
	case reflect.Slice:
		slice := reflect.MakeSlice(v.Type(), 1, 1)
		fillNonZero(t, slice.Index(0), depth)
		v.Set(slice)
	case reflect.Pointer:
		pointer := reflect.New(v.Type().Elem())
		fillNonZero(t, pointer.Elem(), depth)
		v.Set(pointer)
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			t.Fatalf("cannot fill map with key type %s", v.Type().Key())
		}
		entries := reflect.MakeMap(v.Type())
		if depth < maxFillDepth {
			entry := reflect.New(v.Type().Elem()).Elem()
			fillNonZero(t, entry, depth+1)
			entries.SetMapIndex(reflect.ValueOf("child"), entry)
		}
		v.Set(entries)
	case reflect.Struct:
		for i := range v.NumField() {
			fillNonZero(t, v.Field(i), depth)
		}
	default:
		t.Fatalf("fillNonZero does not support kind %s (%s)", v.Kind(), v.Type())
	}
}

// requireNoZeroFields fails for every struct field below v that is its zero
// value. Maps and slices must be non-empty. The deepest map level is allowed to
// be empty because fillNonZero stops recursing there.
func requireNoZeroFields(t *testing.T, v reflect.Value, path string) {
	t.Helper()

	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			t.Errorf("%s is nil after round trip", path)

			return
		}
		requireNoZeroFields(t, v.Elem(), path)
	case reflect.Struct:
		for i := range v.NumField() {
			field := v.Type().Field(i)
			fieldPath := path + "." + field.Name
			if v.Field(i).IsZero() {
				t.Errorf("%s was lost in the gob round trip (decoded as zero value)", fieldPath)

				continue
			}
			requireNoZeroFields(t, v.Field(i), fieldPath)
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			requireNoZeroFields(t, v.MapIndex(key), path+"["+key.String()+"]")
		}
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
				IsSet:       true,
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
