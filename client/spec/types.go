package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode"

	"gopkg.in/yaml.v3"
)

// Note: the order of fields here is important for the order of keys
// in the generated JSON. So we don't rely on automatic generation for these
// types:

type Schema struct {
	FormatVersion string  `json:"formatVersion" yaml:"formatVersion"`
	Tables        []Table `json:"tables" yaml:"tables"`
}

type Table struct {
	Name    string   `json:"name" yaml:"name"`
	Columns []Column `json:"columns" yaml:"columns"`
}

type Column struct {
	Name string     `json:"name" validate:"required" yaml:"name"`
	Type ColumnType `json:"type" validate:"required" yaml:"type"`

	Columns []Column      `json:"columns,omitempty" yaml:"columns,omitempty"`
	Object  *ColumnObject `json:"-" yaml:"-"`

	String   *ColumnString   `json:"string,omitempty" yaml:"string,omitempty"`
	Bool     *ColumnBool     `json:"bool,omitempty" yaml:"bool,omitempty"`
	Email    *ColumnEmail    `json:"email,omitempty" yaml:"email,omitempty"`
	Text     *ColumnText     `json:"text,omitempty" yaml:"text,omitempty"`
	Multiple *ColumnMultiple `json:"multiple,omitempty" yaml:"multiple,omitempty"`
	Link     *ColumnLink     `json:"link,omitempty" yaml:"link,omitempty"`
	Int      *ColumnInt      `json:"int,omitempty" yaml:"int,omitempty"`
	Float    *ColumnInt      `json:"float,omitempty" yaml:"float,omitempty"`

	Required bool `json:"required,omitempty" yaml:"required,omitempty"`
	Unique   bool `json:"unique,omitempty" yaml:"unique,omitempty"`

	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// ColumnString contains settings for the string type.
type ColumnString struct {
}

// ColumnEmail contains settings for the email type.
type ColumnEmail struct {
}

// ColumnMultiple contains settings for the multiple type.
type ColumnMultiple struct {
}

// ColumnText contains settings for the text type.
type ColumnText struct {
}

// ColumnBool contains settings for the bool type.
type ColumnBool struct {
}

// ColumnObject contains settings for the object type.
type ColumnObject struct {
	Columns map[string]Column `dynamodbav:"columns,omitempty" json:"columns,omitempty" yaml:"columns,omitempty" validate:"required,dive"`
}

// ColumnLink contains the settings for the link type.
// The column link references the record ID in the reference table only.
// When querying a linked column, an object is returned with the additional
// lookup fields from the reference table being merged into the object.
type ColumnLink struct {
	// ID of the column (constraint)
	ID string `dynamodbav:"id" json:"-" yaml:"-"`

	// Referenced table. A column link
	Table   string `dynamodbav:"table" json:"table" yaml:"table"`
	TableID string `dynamodbav:"tableID" json:"-" yaml:"-"`

	// References columns
	LookupFields  []string          `dynamodbav:"lookupFields,omitempty" json:"lookupFields,omitempty" yaml:"lookupFields,omitempty"`
	LookupColumns map[string]Column `dynamodbav:"lookupColumns,omitempty" json:"-" yaml:"-"`
}

// ColumnInt contains settings for the int type.
type ColumnInt struct {
}

// ColumnFloat contains settings for the float type.
type ColumnFloat struct {
}

// ColumnType is an enum for the available column types.
type ColumnType uint16

const (
	// ColumnTypeString is a string column (size limit 255).
	ColumnTypeString ColumnType = iota + 1
	// ColumnTypeBool is a bool column.
	ColumnTypeBool
	// ColumnTypeObject is an object column.
	ColumnTypeObject
	// ColumnTypeMultiple is an array of strings.
	ColumnTypeMultiple
	// ColumnTypeText is a longer text.
	ColumnTypeText
	// ColumnTypeEmail is an email address.
	ColumnTypeEmail
	// ColumnTypeLink is a link to another table, by ID.
	ColumnTypeLink
	// ColumnTypeInt is a long integer
	ColumnTypeInt
	// ColumnTypeFloat is a double precision float
	ColumnTypeFloat
)

func ColumnTypeFromString(s string) ColumnType {
	var toID = map[string]ColumnType{
		"string":   ColumnTypeString,
		"bool":     ColumnTypeBool,
		"object":   ColumnTypeObject,
		"multiple": ColumnTypeMultiple,
		"email":    ColumnTypeEmail,
		"text":     ColumnTypeText,
		"link":     ColumnTypeLink,
		"int":      ColumnTypeInt,
		"float":    ColumnTypeFloat,
	}
	return toID[s]
}

func (ct ColumnType) String() string {
	var toString = map[ColumnType]string{
		ColumnTypeString:   "string",
		ColumnTypeBool:     "bool",
		ColumnTypeObject:   "object",
		ColumnTypeMultiple: "multiple",
		ColumnTypeEmail:    "email",
		ColumnTypeText:     "text",
		ColumnTypeLink:     "link",
		ColumnTypeInt:      "int",
		ColumnTypeFloat:    "float",
	}
	return toString[ct]
}

// MarshalJSON marshals a ColumnType to json.
func (ct ColumnType) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(ct.String())
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// MarshalYAML marshals a ColumnType to json.
func (ct ColumnType) MarshalYAML() (interface{}, error) {
	return ct.String(), nil
}

func (ct *ColumnType) UnmarshalYAML(value *yaml.Node) error {
	*ct = ColumnTypeFromString(value.Value)
	if *ct == 0 {
		return fmt.Errorf("invalid column type [%s]", value.Value)
	}
	return nil
}

// UnmarshalJSON unmarshals a ColumnType from json.
func (ct *ColumnType) UnmarshalJSON(b []byte) error {

	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	*ct = ColumnTypeFromString(j)
	if *ct == 0 {
		return fmt.Errorf("invalid column type [%s]", j)
	}
	return nil
}

// IsValidIdentifier returns true if id is a valid identifier for the Xata API, false otherwise
func IsValidIdentifier(id string) bool {
	if len(id) == 0 {
		return false
	}

	for i, r := range id {
		if i == 0 {
			if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
				return false
			}
		} else {
			if !unicode.IsLetter(r) && !unicode.IsNumber(r) &&
				r != '-' && r != '_' && r != '~' {
				return false
			}
		}
	}
	return true
}
