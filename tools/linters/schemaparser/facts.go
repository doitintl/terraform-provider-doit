package schemaparser

import (
	"bytes"
	"encoding/gob"
	"sort"
)

var (
	_ gob.GobEncoder = (*SchemaFacts)(nil)
	_ gob.GobDecoder = (*SchemaFacts)(nil)
)

// schemaFactsWire is the deterministic wire representation of SchemaFacts.
// Maps are represented as key-sorted slices because gob does not guarantee a
// stable iteration order when encoding maps.
type schemaFactsWire struct {
	SchemasPresent bool
	Schemas        []schemaEntryWire
}

type schemaEntryWire struct {
	Name string
	Info *schemaInfoWire
}

type schemaInfoWire struct {
	FuncName     string
	AttrsPresent bool
	Attrs        []attrEntryWire
}

type attrEntryWire struct {
	Name string
	Info *attrInfoWire
}

type attrInfoWire struct {
	Class                  FieldClass
	IsList                 bool
	HasDefault             bool
	DefaultValue           any
	NestedAttrsPresent     bool
	NestedAttrs            []attrEntryWire
	PlanModifiers          []string
	NotClearable           bool
	RequiresReplaceOnClear bool
}

// GobEncode implements gob.GobEncoder with a deterministic representation.
func (sf *SchemaFacts) GobEncode() ([]byte, error) {
	wire := schemaFactsWire{
		SchemasPresent: sf.Schemas != nil,
		Schemas:        encodeSchemas(sf.Schemas),
	}

	var buffer bytes.Buffer
	if err := gob.NewEncoder(&buffer).Encode(wire); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// GobDecode implements gob.GobDecoder for the deterministic representation.
func (sf *SchemaFacts) GobDecode(data []byte) error {
	var wire schemaFactsWire
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&wire); err != nil {
		return err
	}

	sf.Schemas = decodeSchemas(wire.Schemas, wire.SchemasPresent)

	return nil
}

func encodeSchemas(schemas map[string]*SchemaInfo) []schemaEntryWire {
	entries := make([]schemaEntryWire, 0, len(schemas))
	for _, name := range sortedKeys(schemas) {
		entries = append(entries, schemaEntryWire{
			Name: name,
			Info: encodeSchemaInfo(schemas[name]),
		})
	}

	return entries
}

func encodeSchemaInfo(info *SchemaInfo) *schemaInfoWire {
	if info == nil {
		return nil
	}

	return &schemaInfoWire{
		FuncName:     info.FuncName,
		AttrsPresent: info.Attrs != nil,
		Attrs:        encodeAttrs(info.Attrs),
	}
}

func encodeAttrs(attrs map[string]*AttrInfo) []attrEntryWire {
	entries := make([]attrEntryWire, 0, len(attrs))
	for _, name := range sortedKeys(attrs) {
		entries = append(entries, attrEntryWire{
			Name: name,
			Info: encodeAttrInfo(attrs[name]),
		})
	}

	return entries
}

func encodeAttrInfo(info *AttrInfo) *attrInfoWire {
	if info == nil {
		return nil
	}

	return &attrInfoWire{
		Class:                  info.Class,
		IsList:                 info.IsList,
		HasDefault:             info.HasDefault,
		DefaultValue:           info.DefaultValue,
		NestedAttrsPresent:     info.NestedAttrs != nil,
		NestedAttrs:            encodeAttrs(info.NestedAttrs),
		PlanModifiers:          info.PlanModifiers,
		NotClearable:           info.NotClearable,
		RequiresReplaceOnClear: info.RequiresReplaceOnClear,
	}
}

func decodeSchemas(entries []schemaEntryWire, present bool) map[string]*SchemaInfo {
	if !present {
		return nil
	}

	schemas := make(map[string]*SchemaInfo, len(entries))
	for _, entry := range entries {
		schemas[entry.Name] = decodeSchemaInfo(entry.Info)
	}

	return schemas
}

func decodeSchemaInfo(info *schemaInfoWire) *SchemaInfo {
	if info == nil {
		return nil
	}

	return &SchemaInfo{
		FuncName: info.FuncName,
		Attrs:    decodeAttrs(info.Attrs, info.AttrsPresent),
	}
}

func decodeAttrs(entries []attrEntryWire, present bool) map[string]*AttrInfo {
	if !present {
		return nil
	}

	attrs := make(map[string]*AttrInfo, len(entries))
	for _, entry := range entries {
		attrs[entry.Name] = decodeAttrInfo(entry.Info)
	}

	return attrs
}

func decodeAttrInfo(info *attrInfoWire) *AttrInfo {
	if info == nil {
		return nil
	}

	return &AttrInfo{
		Class:                  info.Class,
		IsList:                 info.IsList,
		HasDefault:             info.HasDefault,
		DefaultValue:           info.DefaultValue,
		NestedAttrs:            decodeAttrs(info.NestedAttrs, info.NestedAttrsPresent),
		PlanModifiers:          info.PlanModifiers,
		NotClearable:           info.NotClearable,
		RequiresReplaceOnClear: info.RequiresReplaceOnClear,
	}
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	return keys
}
