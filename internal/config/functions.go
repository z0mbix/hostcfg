package config

import (
	"bytes"
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
