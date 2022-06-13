package hclencoder

import (
	"fmt"
	"reflect"
	"unicode/utf8"

	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/hcl/hcl/token"
)

type cursor token.Pos

func (c cursor) pos() token.Pos {
	return token.Pos(c)
}

func (c cursor) crlf() cursor {
	c.Line++
	c.Column = 1
	return c
}

func (c cursor) in(step int) cursor {
	c.Offset += step
	return c
}

func (c cursor) out(step int) cursor {
	c.Offset -= step
	return c
}

var startingCursor = cursor{
	Offset: 0,
	Line:   1,
	Column: 1,
}

func positionNodes(node ast.Node, cur cursor, step int) (cursor, error) {
	var err error

	switch node := node.(type) {
	case *ast.LiteralType:
		node.Token.Pos = cur.pos()
		cur.Column += utf8.RuneCountInString(node.Token.Text)
		return cur, nil

	case *ast.ListType:
		node.Lbrack = cur.pos()
		if len(node.List) > 1 {
			cur = cur.crlf().in(step)
		}
		for _, item := range node.List {
			if cur, err = positionNodes(item, cur, step); err != nil {
				return cur, err
			}
			if len(node.List) > 1 {
				cur = cur.crlf()
			}
		}
		cur = cur.out(step)
		node.Rbrack = cur.pos()
		cur.Column++
		return cur, nil

	case *ast.ObjectItem:
		for _, key := range node.Keys {
			key.Token.Pos = cur.pos()
			cur.Column += 1 + utf8.RuneCountInString(node.Keys[0].Token.Text)
		}

		if _, ok := node.Val.(*ast.ObjectType); !ok {
			node.Assign = cur.pos()
		}
		cur.Column += 2

		return positionNodes(node.Val, cur, step)

	case *ast.ObjectList:
		for _, item := range node.Items {
			cur, err = positionNodes(item, cur, step)
			if err != nil {
				return cur, err
			}
			cur = cur.crlf()
		}
		return cur, nil

	case *ast.ObjectType:
		node.Lbrace = cur.pos()
		cur = cur.crlf().in(step)

		if cur, err = positionNodes(node.List, cur, step); err != nil {
			return cur, err
		}
		cur = cur.out(step)
		node.Rbrace = cur.pos()
		cur.Column++
		return cur, nil

	case *ast.File:
		return positionNodes(node.Node, cur, step)

	default:
		return cur, fmt.Errorf("unknown node kind %s", reflect.ValueOf(node).Kind())
	}
}
