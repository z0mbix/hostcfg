package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/typeexpr"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

// ParseTypeConstraint parses a type expression into a cty.Type.
// Returns cty.DynamicPseudoType if expr is nil or represents a missing attribute.
func ParseTypeConstraint(expr hcl.Expression) (cty.Type, hcl.Diagnostics) {
	if expr == nil {
		return cty.DynamicPseudoType, nil
	}

	// Check for empty/missing expression (gohcl creates a static expr with empty range for missing optional attrs)
	exprRange := expr.Range()
	if exprRange.Start.Line == exprRange.End.Line &&
		exprRange.Start.Column == exprRange.End.Column {
		return cty.DynamicPseudoType, nil
	}

	ty, diags := typeexpr.TypeConstraint(expr)
	if diags.HasErrors() {
		return cty.DynamicPseudoType, diags
	}

	return ty, nil
}

// ValidateValue checks if a value conforms to a type constraint.
// Returns the converted value and any diagnostics.
// If the constraint is cty.DynamicPseudoType, any value is accepted.
func ValidateValue(val cty.Value, constraint cty.Type, varName string, declRange *hcl.Range) (cty.Value, hcl.Diagnostics) {
	// Dynamic type accepts anything
	if constraint == cty.DynamicPseudoType {
		return val, nil
	}

	// Null values are allowed for any type
	if val.IsNull() {
		return cty.NullVal(constraint), nil
	}

	// Attempt conversion
	converted, err := convert.Convert(val, constraint)
	if err != nil {
		diag := &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("Invalid value for variable %q", varName),
			Detail: fmt.Sprintf(
				"The given value is not valid for variable %q: %s.\n\nExpected type: %s\nGiven type: %s",
				varName, err.Error(), constraint.FriendlyName(), val.Type().FriendlyName(),
			),
		}
		if declRange != nil {
			diag.Subject = declRange
		}
		return cty.NilVal, hcl.Diagnostics{diag}
	}

	return converted, nil
}

// CoerceStringValue attempts to parse a string into the target type.
// This is used for CLI -e flag values which are always strings.
func CoerceStringValue(str string, targetType cty.Type) (cty.Value, error) {
	// Dynamic type - keep as string
	if targetType == cty.DynamicPseudoType {
		return cty.StringVal(str), nil
	}

	// String type - no conversion needed
	if targetType == cty.String {
		return cty.StringVal(str), nil
	}

	// Boolean type
	if targetType == cty.Bool {
		return coerceToBool(str)
	}

	// Number type
	if targetType == cty.Number {
		return coerceToNumber(str)
	}

	// Complex types (list, set, map, object, tuple) - parse as JSON/HCL
	if targetType.IsListType() || targetType.IsSetType() || targetType.IsMapType() ||
		targetType.IsObjectType() || targetType.IsTupleType() {
		return coerceToComplexType(str, targetType)
	}

	return cty.NilVal, fmt.Errorf("unsupported type conversion to %s", targetType.FriendlyName())
}

// coerceToBool converts a string to a boolean value.
func coerceToBool(str string) (cty.Value, error) {
	lower := strings.ToLower(strings.TrimSpace(str))
	switch lower {
	case "true", "1", "yes", "on":
		return cty.BoolVal(true), nil
	case "false", "0", "no", "off":
		return cty.BoolVal(false), nil
	default:
		return cty.NilVal, fmt.Errorf("cannot convert %q to bool: expected true/false, yes/no, on/off, or 1/0", str)
	}
}

// coerceToNumber converts a string to a number value.
func coerceToNumber(str string) (cty.Value, error) {
	str = strings.TrimSpace(str)

	// Try parsing as integer first
	if i, err := strconv.ParseInt(str, 10, 64); err == nil {
		return cty.NumberIntVal(i), nil
	}

	// Try parsing as float
	if f, err := strconv.ParseFloat(str, 64); err == nil {
		return cty.NumberFloatVal(f), nil
	}

	return cty.NilVal, fmt.Errorf("cannot convert %q to number: not a valid numeric value", str)
}

// coerceToComplexType parses a string as JSON or HCL and converts to the target type.
func coerceToComplexType(str string, targetType cty.Type) (cty.Value, error) {
	str = strings.TrimSpace(str)

	// Try parsing as JSON first (more common for CLI usage)
	val, err := parseJSON(str, targetType)
	if err == nil {
		return val, nil
	}

	// Fall back to HCL syntax
	val, hclErr := parseHCLLiteral(str)
	if hclErr == nil {
		// Convert to target type
		converted, convErr := convert.Convert(val, targetType)
		if convErr == nil {
			return converted, nil
		}
		return cty.NilVal, fmt.Errorf("value parsed but type mismatch: %s", convErr.Error())
	}

	return cty.NilVal, fmt.Errorf("cannot parse %q as %s: invalid JSON or HCL syntax", str, targetType.FriendlyName())
}

// parseJSON parses a JSON string into a cty.Value of the target type.
func parseJSON(str string, targetType cty.Type) (cty.Value, error) {
	// Use cty's JSON unmarshaler
	var jsonVal interface{}
	if err := json.Unmarshal([]byte(str), &jsonVal); err != nil {
		return cty.NilVal, err
	}

	// Convert JSON to cty.Value using implied type, then convert to target
	impliedType, err := ctyjson.ImpliedType([]byte(str))
	if err != nil {
		return cty.NilVal, err
	}

	val, err := ctyjson.Unmarshal([]byte(str), impliedType)
	if err != nil {
		return cty.NilVal, err
	}

	// Convert to target type
	converted, err := convert.Convert(val, targetType)
	if err != nil {
		return cty.NilVal, fmt.Errorf("JSON value cannot be converted to %s: %s", targetType.FriendlyName(), err.Error())
	}

	return converted, nil
}

// parseHCLLiteral parses a string as an HCL literal expression.
func parseHCLLiteral(str string) (cty.Value, error) {
	// Parse as HCL expression
	expr, diags := hclsyntax.ParseExpression([]byte(str), "cli", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return cty.NilVal, fmt.Errorf("invalid HCL syntax: %s", diags.Error())
	}

	// Evaluate the expression with no context (literal only)
	val, diags := expr.Value(nil)
	if diags.HasErrors() {
		return cty.NilVal, fmt.Errorf("cannot evaluate expression: %s", diags.Error())
	}

	return val, nil
}

// GetVariableTypes returns a map of variable names to their type constraints.
// Variables without explicit types are not included in the map.
func GetVariableTypes(variables []*Variable) (map[string]cty.Type, hcl.Diagnostics) {
	result := make(map[string]cty.Type)
	var diags hcl.Diagnostics

	for _, v := range variables {
		if v.TypeExpr != nil {
			ty, typeDiags := ParseTypeConstraint(v.TypeExpr)
			diags = append(diags, typeDiags...)
			if !typeDiags.HasErrors() {
				result[v.Name] = ty
			}
		}
	}

	return result, diags
}
