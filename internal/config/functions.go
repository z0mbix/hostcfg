package config

import (
	"os"
	"path/filepath"
	"strings"

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
