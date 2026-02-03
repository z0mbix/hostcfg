package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

// standardFunctions returns the standard set of functions available in HCL
func standardFunctions() map[string]function.Function {
	return map[string]function.Function{
		// String functions
		"upper":      stdlib.UpperFunc,
		"lower":      stdlib.LowerFunc,
		"trim":       stdlib.TrimFunc,
		"trimprefix": stdlib.TrimPrefixFunc,
		"trimsuffix": stdlib.TrimSuffixFunc,
		"trimspace":  stdlib.TrimSpaceFunc,
		"replace":    stdlib.ReplaceFunc,
		"substr":     stdlib.SubstrFunc,
		"join":       stdlib.JoinFunc,
		"split":      stdlib.SplitFunc,
		"format":     stdlib.FormatFunc,
		"formatlist": stdlib.FormatListFunc,

		// Collection functions
		"length":   stdlib.LengthFunc,
		"coalesce": stdlib.CoalesceFunc,
		"concat":   stdlib.ConcatFunc,
		"contains": stdlib.ContainsFunc,
		"distinct": stdlib.DistinctFunc,
		"flatten":  stdlib.FlattenFunc,
		"keys":     stdlib.KeysFunc,
		"values":   stdlib.ValuesFunc,
		"merge":    stdlib.MergeFunc,
		"reverse":  stdlib.ReverseListFunc,
		"sort":     stdlib.SortFunc,

		// Numeric functions
		"abs":   stdlib.AbsoluteFunc,
		"ceil":  stdlib.CeilFunc,
		"floor": stdlib.FloorFunc,
		"max":   stdlib.MaxFunc,
		"min":   stdlib.MinFunc,

		// Boolean functions
		"not": stdlib.NotFunc,
		"and": stdlib.AndFunc,
		"or":  stdlib.OrFunc,

		// Type conversion
		"tostring": stdlib.MakeToFunc(cty.String),
		"tonumber": stdlib.MakeToFunc(cty.Number),
		"tobool":   stdlib.MakeToFunc(cty.Bool),

		// Custom functions
		"env":      envFunc,
		"file":     fileFunc,
		"basename": basenameFunc,
		"dirname":  dirnameFunc,

		// Type conversion functions for for_each
		"toset": tosetFunc,
		"tomap": tomapFunc,
	}
}

// envFunc returns the value of an environment variable
var envFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name: "name",
			Type: cty.String,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		name := args[0].AsString()
		return cty.StringVal(os.Getenv(name)), nil
	},
})

// fileFunc reads the contents of a file
var fileFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name: "path",
			Type: cty.String,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		path := args[0].AsString()
		content, err := os.ReadFile(path)
		if err != nil {
			return cty.StringVal(""), err
		}
		return cty.StringVal(strings.TrimSuffix(string(content), "\n")), nil
	},
})

// basenameFunc returns the base name of a path
var basenameFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name: "path",
			Type: cty.String,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		path := args[0].AsString()
		return cty.StringVal(filepath.Base(path)), nil
	},
})

// dirnameFunc returns the directory name of a path
var dirnameFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name: "path",
			Type: cty.String,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		path := args[0].AsString()
		return cty.StringVal(filepath.Dir(path)), nil
	},
})

// tosetFunc converts a list/tuple to a set (for use with for_each)
var tosetFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name: "list",
			Type: cty.DynamicPseudoType,
		},
	},
	Type: func(args []cty.Value) (cty.Type, error) {
		listVal := args[0]
		if listVal.Type().IsListType() {
			return cty.Set(listVal.Type().ElementType()), nil
		}
		if listVal.Type().IsTupleType() {
			// For tuples, use string as the element type (safest common type)
			return cty.Set(cty.String), nil
		}
		if listVal.Type().IsSetType() {
			// Already a set, return as-is
			return listVal.Type(), nil
		}
		return cty.NilType, fmt.Errorf("toset requires a list, tuple, or set; got %s", listVal.Type().FriendlyName())
	},
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		listVal := args[0]

		if listVal.IsNull() {
			return cty.NullVal(retType), nil
		}
		if !listVal.IsKnown() {
			return cty.UnknownVal(retType), nil
		}

		// If already a set, return it
		if listVal.Type().IsSetType() {
			return listVal, nil
		}

		// Convert list/tuple to set
		var vals []cty.Value
		it := listVal.ElementIterator()
		for it.Next() {
			_, v := it.Element()
			// For tuples with mixed types, convert to string
			if listVal.Type().IsTupleType() && v.Type() != cty.String {
				v = cty.StringVal(v.GoString())
			}
			vals = append(vals, v)
		}

		if len(vals) == 0 {
			return cty.SetValEmpty(retType.ElementType()), nil
		}

		return cty.SetVal(vals), nil
	},
})

// tomapFunc converts an object to a map (for use with for_each)
var tomapFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name: "value",
			Type: cty.DynamicPseudoType,
		},
	},
	Type: func(args []cty.Value) (cty.Type, error) {
		val := args[0]
		if val.Type().IsMapType() {
			return val.Type(), nil
		}
		if val.Type().IsObjectType() {
			// Convert object to map of dynamic
			return cty.Map(cty.DynamicPseudoType), nil
		}
		return cty.NilType, fmt.Errorf("tomap requires a map or object; got %s", val.Type().FriendlyName())
	},
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		val := args[0]

		if val.IsNull() {
			return cty.NullVal(retType), nil
		}
		if !val.IsKnown() {
			return cty.UnknownVal(retType), nil
		}

		// If already a map, return it
		if val.Type().IsMapType() {
			return val, nil
		}

		// Convert object to map
		if val.Type().IsObjectType() {
			return cty.MapVal(val.AsValueMap()), nil
		}

		return cty.NilVal, fmt.Errorf("cannot convert %s to map", val.Type().FriendlyName())
	},
})

// ctyToGoMap converts a cty.Value (object/map) to a Go map for template execution
func ctyToGoMap(val cty.Value) map[string]interface{} {
	result := make(map[string]interface{})
	if val.IsNull() || !val.IsKnown() {
		return result
	}

	if val.Type().IsObjectType() || val.Type().IsMapType() {
		for k, v := range val.AsValueMap() {
			result[k] = ctyToInterface(v)
		}
	}
	return result
}

// ctyToInterface converts a cty.Value to a Go interface{} for template execution
func ctyToInterface(val cty.Value) interface{} {
	if val.IsNull() || !val.IsKnown() {
		return nil
	}
	switch {
	case val.Type() == cty.String:
		return val.AsString()
	case val.Type() == cty.Number:
		f, _ := val.AsBigFloat().Float64()
		return f
	case val.Type() == cty.Bool:
		return val.True()
	case val.Type().IsObjectType() || val.Type().IsMapType():
		return ctyToGoMap(val)
	case val.Type().IsListType() || val.Type().IsTupleType():
		var list []interface{}
		for _, item := range val.AsValueSlice() {
			list = append(list, ctyToInterface(item))
		}
		return list
	default:
		return val.GoString()
	}
}

// makeTemplateFunc creates a template function with access to context variables
// baseDir is the directory containing the HCL files, used to resolve relative template paths
// Templates have access to all Sprig functions (https://masterminds.github.io/sprig/)
func makeTemplateFunc(ctxVars map[string]cty.Value, baseDir string) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{Name: "path", Type: cty.String},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			path := args[0].AsString()

			// Resolve relative paths against the HCL config directory
			if !filepath.IsAbs(path) && baseDir != "" {
				path = filepath.Join(baseDir, path)
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return cty.StringVal(""), err
			}

			// Convert all context variables to Go map
			tmplVars := make(map[string]interface{})
			for k, v := range ctxVars {
				tmplVars[k] = ctyToGoMap(v)
			}

			// Create template with Sprig functions
			tmpl, err := template.New(filepath.Base(path)).Funcs(sprig.FuncMap()).Parse(string(content))
			if err != nil {
				return cty.StringVal(""), err
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, tmplVars); err != nil {
				return cty.StringVal(""), err
			}

			return cty.StringVal(buf.String()), nil
		},
	})
}
