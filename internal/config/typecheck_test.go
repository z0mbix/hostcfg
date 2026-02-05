package config

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// parseTypeExpr is a helper to parse a type expression string
func parseTypeExpr(t *testing.T, typeStr string) hcl.Expression {
	t.Helper()
	expr, diags := hclsyntax.ParseExpression([]byte(typeStr), "test", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		t.Fatalf("failed to parse type expression %q: %s", typeStr, diags.Error())
	}
	return expr
}

func TestParseTypeConstraint_BasicTypes(t *testing.T) {
	tests := []struct {
		name     string
		typeExpr string
		want     cty.Type
	}{
		{"string", "string", cty.String},
		{"number", "number", cty.Number},
		{"bool", "bool", cty.Bool},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := parseTypeExpr(t, tt.typeExpr)
			got, diags := ParseTypeConstraint(expr)
			if diags.HasErrors() {
				t.Fatalf("ParseTypeConstraint() error: %s", diags.Error())
			}
			if !got.Equals(tt.want) {
				t.Errorf("ParseTypeConstraint() = %s, want %s", got.FriendlyName(), tt.want.FriendlyName())
			}
		})
	}
}

func TestParseTypeConstraint_ComplexTypes(t *testing.T) {
	tests := []struct {
		name     string
		typeExpr string
		check    func(cty.Type) bool
	}{
		{
			name:     "list of strings",
			typeExpr: "list(string)",
			check: func(ty cty.Type) bool {
				return ty.IsListType() && ty.ElementType().Equals(cty.String)
			},
		},
		{
			name:     "set of numbers",
			typeExpr: "set(number)",
			check: func(ty cty.Type) bool {
				return ty.IsSetType() && ty.ElementType().Equals(cty.Number)
			},
		},
		{
			name:     "map of strings",
			typeExpr: "map(string)",
			check: func(ty cty.Type) bool {
				return ty.IsMapType() && ty.ElementType().Equals(cty.String)
			},
		},
		{
			name:     "object type",
			typeExpr: "object({ name = string, port = number })",
			check: func(ty cty.Type) bool {
				if !ty.IsObjectType() {
					return false
				}
				attrTypes := ty.AttributeTypes()
				return attrTypes["name"].Equals(cty.String) && attrTypes["port"].Equals(cty.Number)
			},
		},
		{
			name:     "nested list of maps",
			typeExpr: "list(map(string))",
			check: func(ty cty.Type) bool {
				if !ty.IsListType() {
					return false
				}
				elemType := ty.ElementType()
				return elemType.IsMapType() && elemType.ElementType().Equals(cty.String)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := parseTypeExpr(t, tt.typeExpr)
			got, diags := ParseTypeConstraint(expr)
			if diags.HasErrors() {
				t.Fatalf("ParseTypeConstraint() error: %s", diags.Error())
			}
			if !tt.check(got) {
				t.Errorf("ParseTypeConstraint() returned unexpected type: %s", got.FriendlyName())
			}
		})
	}
}

func TestParseTypeConstraint_NilExpr(t *testing.T) {
	got, diags := ParseTypeConstraint(nil)
	if diags.HasErrors() {
		t.Fatalf("ParseTypeConstraint(nil) error: %s", diags.Error())
	}
	if got != cty.DynamicPseudoType {
		t.Errorf("ParseTypeConstraint(nil) = %s, want DynamicPseudoType", got.FriendlyName())
	}
}

func TestValidateValue_Match(t *testing.T) {
	tests := []struct {
		name       string
		value      cty.Value
		constraint cty.Type
	}{
		{"string matches string", cty.StringVal("hello"), cty.String},
		{"number matches number", cty.NumberIntVal(42), cty.Number},
		{"bool matches bool", cty.BoolVal(true), cty.Bool},
		{"list matches list", cty.ListVal([]cty.Value{cty.StringVal("a")}), cty.List(cty.String)},
		{"any type with dynamic", cty.StringVal("anything"), cty.DynamicPseudoType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, diags := ValidateValue(tt.value, tt.constraint, "test_var", nil)
			if diags.HasErrors() {
				t.Fatalf("ValidateValue() error: %s", diags.Error())
			}
			if got.IsNull() {
				t.Error("ValidateValue() returned null value")
			}
		})
	}
}

func TestValidateValue_Mismatch(t *testing.T) {
	tests := []struct {
		name       string
		value      cty.Value
		constraint cty.Type
	}{
		{"string vs number", cty.StringVal("hello"), cty.Number},
		{"number vs bool", cty.NumberIntVal(42), cty.Bool},
		{"string vs list", cty.StringVal("hello"), cty.List(cty.String)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, diags := ValidateValue(tt.value, tt.constraint, "test_var", nil)
			if !diags.HasErrors() {
				t.Error("ValidateValue() expected error for type mismatch")
			}
		})
	}
}

func TestValidateValue_NullAllowed(t *testing.T) {
	// Null values should be allowed for any type
	got, diags := ValidateValue(cty.NullVal(cty.String), cty.Number, "test_var", nil)
	if diags.HasErrors() {
		t.Fatalf("ValidateValue() error for null: %s", diags.Error())
	}
	if !got.IsNull() {
		t.Error("ValidateValue() should return null for null input")
	}
}

func TestValidateValue_Coercion(t *testing.T) {
	// Integer should be coercible to number
	got, diags := ValidateValue(cty.NumberIntVal(42), cty.Number, "test_var", nil)
	if diags.HasErrors() {
		t.Fatalf("ValidateValue() error: %s", diags.Error())
	}
	if !got.Type().Equals(cty.Number) {
		t.Errorf("ValidateValue() type = %s, want number", got.Type().FriendlyName())
	}
}

func TestCoerceStringValue_Bool(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"1", true},
		{"yes", true},
		{"on", true},
		{"false", false},
		{"False", false},
		{"FALSE", false},
		{"0", false},
		{"no", false},
		{"off", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := CoerceStringValue(tt.input, cty.Bool)
			if err != nil {
				t.Fatalf("CoerceStringValue() error: %v", err)
			}
			if got.False() == tt.want {
				t.Errorf("CoerceStringValue(%q) = %v, want %v", tt.input, got.True(), tt.want)
			}
		})
	}
}

func TestCoerceStringValue_Bool_Invalid(t *testing.T) {
	_, err := CoerceStringValue("maybe", cty.Bool)
	if err == nil {
		t.Error("CoerceStringValue() expected error for invalid bool")
	}
}

func TestCoerceStringValue_Number(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"42", 42},
		{"0", 0},
		{"-10", -10},
		{"1000000", 1000000},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := CoerceStringValue(tt.input, cty.Number)
			if err != nil {
				t.Fatalf("CoerceStringValue() error: %v", err)
			}
			gotInt, _ := got.AsBigFloat().Int64()
			if gotInt != tt.want {
				t.Errorf("CoerceStringValue(%q) = %d, want %d", tt.input, gotInt, tt.want)
			}
		})
	}
}

func TestCoerceStringValue_Number_Float(t *testing.T) {
	got, err := CoerceStringValue("3.14", cty.Number)
	if err != nil {
		t.Fatalf("CoerceStringValue() error: %v", err)
	}
	gotFloat, _ := got.AsBigFloat().Float64()
	if gotFloat != 3.14 {
		t.Errorf("CoerceStringValue(\"3.14\") = %f, want 3.14", gotFloat)
	}
}

func TestCoerceStringValue_Number_Invalid(t *testing.T) {
	_, err := CoerceStringValue("not-a-number", cty.Number)
	if err == nil {
		t.Error("CoerceStringValue() expected error for invalid number")
	}
}

func TestCoerceStringValue_String(t *testing.T) {
	got, err := CoerceStringValue("hello", cty.String)
	if err != nil {
		t.Fatalf("CoerceStringValue() error: %v", err)
	}
	if got.AsString() != "hello" {
		t.Errorf("CoerceStringValue(\"hello\") = %q, want \"hello\"", got.AsString())
	}
}

func TestCoerceStringValue_List_JSON(t *testing.T) {
	got, err := CoerceStringValue(`["a", "b", "c"]`, cty.List(cty.String))
	if err != nil {
		t.Fatalf("CoerceStringValue() error: %v", err)
	}
	if !got.Type().IsListType() {
		t.Errorf("CoerceStringValue() type = %s, want list", got.Type().FriendlyName())
	}
	if got.LengthInt() != 3 {
		t.Errorf("CoerceStringValue() length = %d, want 3", got.LengthInt())
	}
}

func TestCoerceStringValue_Map_JSON(t *testing.T) {
	got, err := CoerceStringValue(`{"key": "value"}`, cty.Map(cty.String))
	if err != nil {
		t.Fatalf("CoerceStringValue() error: %v", err)
	}
	if !got.Type().IsMapType() {
		t.Errorf("CoerceStringValue() type = %s, want map", got.Type().FriendlyName())
	}
}

func TestCoerceStringValue_Object_JSON(t *testing.T) {
	objType := cty.Object(map[string]cty.Type{
		"name": cty.String,
		"port": cty.Number,
	})
	got, err := CoerceStringValue(`{"name": "test", "port": 8080}`, objType)
	if err != nil {
		t.Fatalf("CoerceStringValue() error: %v", err)
	}
	if !got.Type().IsObjectType() {
		t.Errorf("CoerceStringValue() type = %s, want object", got.Type().FriendlyName())
	}
}

func TestCoerceStringValue_DynamicKeepsString(t *testing.T) {
	got, err := CoerceStringValue("hello", cty.DynamicPseudoType)
	if err != nil {
		t.Fatalf("CoerceStringValue() error: %v", err)
	}
	if !got.Type().Equals(cty.String) {
		t.Errorf("CoerceStringValue() with dynamic type = %s, want string", got.Type().FriendlyName())
	}
	if got.AsString() != "hello" {
		t.Errorf("CoerceStringValue() = %q, want \"hello\"", got.AsString())
	}
}
