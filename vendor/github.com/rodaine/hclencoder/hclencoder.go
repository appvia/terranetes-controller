package hclencoder

import (
	"bytes"
	"reflect"

	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/hcl/hcl/printer"
)

// Encode converts any supported type into the corresponding HCL format
func Encode(in interface{}) ([]byte, error) {
	node, _, err := encode(reflect.ValueOf(in))
	if err != nil {
		return nil, err
	}

	file := &ast.File{}
	switch node := node.(type) {
	case *ast.ObjectType:
		file.Node = node.List
	default:
		file.Node = node
	}

	if _, err = positionNodes(file, startingCursor, 2); err != nil {
		return nil, err
	}

	b := &bytes.Buffer{}
	err = printer.Fprint(b, file)
	b.WriteString("\n")

	return b.Bytes(), err
}
