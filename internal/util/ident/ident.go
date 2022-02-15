// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

// Package ident contains types for safely representing SQL identifiers.
package ident

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Well-known identifiers.
var (
	StagingDB = New("_cdc_sink") // "_cdc_sink"
	Public    = New("public")    // "public"
)

// An Ident is a quoted SQL identifier, generally a table, column, or
// database. This type is an immutable value type, suitable for use as a
// map key.
type Ident struct {
	q, r string
}

// New returns a quoted SQL identifier. This method will panic if an
// empty string is passed.
func New(raw string) Ident {
	return Ident{`"` + strings.ReplaceAll(raw, `"`, `""`) + `"`, raw}
}

// Newf returns a quoted SQL identifier.
func Newf(format string, args ...interface{}) Ident {
	return New(fmt.Sprintf(format, args...))
}

// Qualification is a return value from Relative, indicating how many
// name parts were present in the initial input.
type Qualification int

//go:generate go run golang.org/x/tools/cmd/stringer -type=Qualification

// Various levels of table-identifier qualification.
const (
	TableOnly Qualification = iota + 1
	TableAndDatabase
	FullyQualified
)

// Relative parses a table name and returns a fully-qualified Table
// name in the given database and table schema.
func Relative(db Ident, schema Ident, table string) (Table, Qualification, error) {
	parts := strings.Split(table, ".")
	switch len(parts) {
	case 1:
		if parts[0] == "" {
			return Table{}, 0, errors.New("empty table")
		}
		return Table{db, schema, New(parts[0])}, 1, nil
	case 2:
		if parts[0] == "" {
			return Table{}, 0, errors.New("empty database")
		}
		if parts[1] == "" {
			return Table{}, 0, errors.New("empty table")
		}
		return Table{db, schema, New(parts[1])}, 2, nil
	case 3:
		if parts[0] == "" {
			return Table{}, 0, errors.New("empty database")
		}
		if parts[1] == "" {
			return Table{}, 0, errors.New("empty schema")
		}
		if parts[2] == "" {
			return Table{}, 0, errors.New("empty table")
		}
		return Table{db, schema, New(parts[2])}, 3, nil
	default:
		return Table{}, 0, errors.Errorf("too many parts in %q", table)
	}
}

// IsEmpty returns true if the identifier is empty.
func (n Ident) IsEmpty() bool {
	return n.r == ""
}

// IsZero returns IsEmpty().
func (n Ident) IsZero() bool {
	return n.IsEmpty()
}

// MarshalJSON returns the Ident's raw form.
func (n Ident) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.Raw())
}

// MarshalYAML returns the Ident's raw form.
func (n Ident) MarshalYAML() (interface{}, error) {
	return n.Raw(), nil
}

// Raw returns the original, raw value.
func (n Ident) Raw() string {
	return n.r
}

// Scan implements sql.Scanner to allow an Ident to be retrieved from a
// string value.
func (n *Ident) Scan(value interface{}) error {
	if value == nil {
		n.q = ""
		n.r = ""
		return nil
	}
	if s, ok := value.(string); ok {
		*n = New(s)
		return nil
	}
	return errors.Errorf("cannot scan an Ident from a %T", value)
}

// String returns the ident in a manner suitable for constructing a query.
func (n Ident) String() string { return n.q }

// UnmarshalJSON converts a raw json string into an Ident.
func (n *Ident) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*n = New(raw)
	return nil
}

// UnmarshalYAML converts a raw yaml string into an Ident.
func (n *Ident) UnmarshalYAML(node *yaml.Node) error {
	*n = New(node.Value)
	return nil
}

// Value implements driver.Valuer, allowing the Ident to be persisted
// as its raw-string value.
func (n Ident) Value() (driver.Value, error) {
	return n.Raw(), nil
}

// A Schema identifier is a two-part ident, consisting of an SQL
// database and schema. This type is an immutable value
// type, suitable for use as a map key.
type Schema struct {
	db, schema Ident
}

// NewSchema constructs a Schema identifier.
func NewSchema(db, schema Ident) Schema {
	return Schema{db, schema}
}

// AsSchema returns the Schema.
func (s Schema) AsSchema() Schema { return s }

// Contains returns true if the given table is defined within the
// user-defined schema.
func (s Schema) Contains(table Table) bool {
	return s.Database() == table.Database() && s.Schema() == table.Schema()
}

// Database returns the schema's enclosing database.
func (s Schema) Database() Ident { return s.db }

// MarshalJSON returns the Schema as a two-element array.
func (s Schema) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string{s.Database().Raw(), s.Schema().Raw()})
}

// MarshalYAML returns the Schema as a two-element, flow-style array.
//   [ database, schema ]
func (s Schema) MarshalYAML() (interface{}, error) {
	return &yaml.Node{
		Kind:  yaml.SequenceNode,
		Style: yaml.FlowStyle,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: s.Database().Raw()},
			{Kind: yaml.ScalarNode, Value: s.Schema().Raw()},
		},
	}, nil
}

// Schema returns the schema's name.
func (s Schema) Schema() Ident { return s.schema }

// Raw returns the original, raw value.
func (s Schema) Raw() string {
	return fmt.Sprintf("%s.%s", s.Database().Raw(), s.Schema().Raw())
}

// String returns the identifier in a manner suitable for constructing a
// query.
func (s Schema) String() string {
	return fmt.Sprintf("%s.%s", s.Database(), s.Schema())
}

// UnmarshalJSON parses a two-element array.
func (s *Schema) UnmarshalJSON(data []byte) error {
	parts := make([]Ident, 0, 2)
	if err := json.Unmarshal(data, &parts); err != nil {
		return err
	}
	if len(parts) != 2 {
		return errors.Errorf("expecting 2 parts, had %d", len(parts))
	}
	s.db = parts[0]
	s.schema = parts[1]
	return nil
}

// UnmarshalYAML parses a two-element array.
func (s *Schema) UnmarshalYAML(node *yaml.Node) error {
	parts := make([]Ident, 0, 2)
	if err := node.Decode(&parts); err != nil {
		return err
	}
	if len(parts) != 2 {
		return errors.Errorf("expecting 2 parts, had %d", len(parts))
	}
	s.db = parts[0]
	s.schema = parts[1]
	return nil
}

// Schematic is anything that can convert itself to a schema.
type Schematic interface {
	// AsSchema returns the value as a Schema.
	AsSchema() Schema
}

var _ Schematic = Schema{}
var _ Schematic = Table{}

// A Table identifier is a three-part ident, consisting of an SQL
// database, schema, and table ident. This type is an immutable value
// type, suitable for use as a map key.
type Table struct {
	db, schema, table Ident
}

// NewTable constructs a Table identifier.
func NewTable(db, schema, table Ident) Table {
	return Table{db, schema, table}
}

// AsSchema returns the schema from the table.
func (t Table) AsSchema() Schema {
	return NewSchema(t.Database(), t.Schema())
}

// Database returns the table's enclosing database.
func (t Table) Database() Ident { return t.db }

// MarshalJSON returns the ident as a three-element array.
func (t Table) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string{t.Database().Raw(), t.Schema().Raw(), t.Table().Raw()})
}

// MarshalYAML returns the Table as a three-element, flow-style array.
//   [ database, schema, table ]
func (t Table) MarshalYAML() (interface{}, error) {
	return &yaml.Node{
		Style: yaml.FlowStyle,
		Kind:  yaml.SequenceNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: t.Database().Raw()},
			{Kind: yaml.ScalarNode, Value: t.Schema().Raw()},
			{Kind: yaml.ScalarNode, Value: t.Table().Raw()},
		},
	}, nil
}

// Schema returns the table's enclosing schema.
func (t Table) Schema() Ident { return t.schema }

// Table returns the table's identifier.
func (t Table) Table() Ident { return t.table }

// Raw returns the original, raw value.
func (t Table) Raw() string {
	return fmt.Sprintf("%s.%s.%s", t.Database().Raw(), t.Schema().Raw(), t.Table().Raw())
}

// String returns the identifier in a manner suitable for constructing a
// query.
func (t Table) String() string {
	return fmt.Sprintf("%s.%s.%s", t.Database(), t.Schema(), t.Table())
}

// UnmarshalJSON parses a three-element array.
func (t *Table) UnmarshalJSON(data []byte) error {
	parts := make([]Ident, 0, 3)
	if err := json.Unmarshal(data, &parts); err != nil {
		return err
	}
	if len(parts) != 3 {
		return errors.Errorf("expecting 3 parts, had %d", len(parts))
	}
	t.db = parts[0]
	t.schema = parts[1]
	t.table = parts[2]
	return nil
}

// UnmarshalYAML parses a three-element array.
func (t *Table) UnmarshalYAML(node *yaml.Node) error {
	parts := make([]Ident, 0, 3)
	if err := node.Decode(&parts); err != nil {
		return err
	}
	if len(parts) != 3 {
		return errors.Errorf("expecting 3 parts, had %d", len(parts))
	}
	t.db = parts[0]
	t.schema = parts[1]
	t.table = parts[2]
	return nil
}
