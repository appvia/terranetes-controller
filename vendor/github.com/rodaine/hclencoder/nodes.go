package hclencoder

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/hcl/hcl/token"
)

const (
	// HCLTagName is the struct field tag used by the HCL decoder. The
	// values from this tag are used in the same way as the decoder.
	HCLTagName = "hcl"

	// KeyTag indicates that the value of the field should be part of
	// the parent object block's key, not a property of that block
	KeyTag string = "key"

	// SquashTag is attached to anonymous fields of a struct and indicates
	// to the encoder to lift the fields of that value into the parent
	// block's scope transparently. Otherwise, the field's type is used as
	// the key for the value.
	SquashTag string = "squash"

	// UnusedKeysTag is a flag that indicates any unused keys found by the
	// decoder are stored in this field of type []string. This has the same
	// behavior as the OmitTag and is not encoded.
	UnusedKeysTag string = "unusedKeys"

	// DecodedFieldsTag is a flag that indicates all fields decoded are
	// stored in this field of type []string. This has the same behavior as
	// the OmitTag and is not encoded.
	DecodedFieldsTag string = "decodedFields"

	// HCLETagName is the struct field tag used by this package. The
	// values from this tag are used in conjunction with HCLTag values.
	HCLETagName = "hcle"

	// OmitTag will omit this field from encoding. This is the similar
	// behavior to `json:"-"`.
	OmitTag string = "omit"

	// OmitEmptyTag will omit this field if it is a zero value. This
	// is similar behavior to `json:",omitempty"`
	OmitEmptyTag string = "omitempty"
)

type fieldMeta struct {
	anonymous     bool
	name          string
	key           bool
	squash        bool
	unusedKeys    bool
	decodedFields bool
	omit          bool
	omitEmpty     bool
}

// encode converts a reflected valued into an HCL ast.Node in a depth-first manner.
func encode(in reflect.Value) (node ast.Node, key []*ast.ObjectKey, err error) {
	in, isNil := deref(in)
	if isNil {
		return nil, nil, nil
	}

	switch in.Kind() {

	case reflect.Bool, reflect.Float64, reflect.String,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return encodePrimitive(in)

	case reflect.Slice:
		return encodeList(in)

	case reflect.Map:
		return encodeMap(in)

	case reflect.Struct:
		return encodeStruct(in)

	default:
		return nil, nil, fmt.Errorf("cannot encode kind %s to HCL", in.Kind())
	}

}

// encodePrimitive converts a primitive value into an ast.LiteralType. An
// ast.ObjectKey is never returned.
func encodePrimitive(in reflect.Value) (ast.Node, []*ast.ObjectKey, error) {
	tkn, err := tokenize(in, false)
	if err != nil {
		return nil, nil, err
	}

	return &ast.LiteralType{Token: tkn}, nil, nil
}

// encodeList converts a slice to an appropriate ast.Node type depending on its
// element value type. An ast.ObjectKey is never returned.
func encodeList(in reflect.Value) (ast.Node, []*ast.ObjectKey, error) {
	childType := in.Type().Elem()

childLoop:
	for {
		switch childType.Kind() {
		case reflect.Ptr:
			childType = childType.Elem()
		default:
			break childLoop
		}
	}

	switch childType.Kind() {
	case reflect.Map, reflect.Struct, reflect.Interface:
		return encodeBlockList(in)
	default:
		return encodePrimitiveList(in)
	}
}

// encodePrimitiveList converts a slice of primitive values to an ast.ListType. An
// ast.ObjectKey is never returned.
func encodePrimitiveList(in reflect.Value) (ast.Node, []*ast.ObjectKey, error) {
	l := in.Len()
	n := &ast.ListType{List: make([]ast.Node, 0, l)}

	for i := 0; i < l; i++ {
		child, _, err := encode(in.Index(i))
		if err != nil {
			return nil, nil, err
		}
		if child != nil {
			n.Add(child)
		}
	}

	return n, nil, nil
}

// encodeBlockList converts a slice of non-primitive types to an ast.ObjectList. An
// ast.ObjectKey is never returned.
func encodeBlockList(in reflect.Value) (ast.Node, []*ast.ObjectKey, error) {
	l := in.Len()
	n := &ast.ObjectList{Items: make([]*ast.ObjectItem, 0, l)}

	for i := 0; i < l; i++ {
		child, childKey, err := encode(in.Index(i))
		if err != nil {
			return nil, nil, err
		}
		if child == nil {
			continue
		}
		if childKey == nil {
			return encodePrimitiveList(in)
		}

		item := &ast.ObjectItem{Val: child}
		item.Keys = childKey
		n.Add(item)
	}

	return n, nil, nil
}

// encodeMap converts a map type into an ast.ObjectType. Maps must have string
// key values to be encoded. An ast.ObjectKey is never returned.
func encodeMap(in reflect.Value) (ast.Node, []*ast.ObjectKey, error) {
	if keyType := in.Type().Key().Kind(); keyType != reflect.String {
		return nil, nil, fmt.Errorf("map keys must be strings, %s given", keyType)
	}

	l := make(objectItems, 0, in.Len())
	for _, key := range in.MapKeys() {
		tkn, _ := tokenize(key, true) // error impossible since we've already checked key kind

		val, childKey, err := encode(in.MapIndex(key))
		if err != nil {
			return nil, nil, err
		}
		if val == nil {
			continue
		}

		switch typ := val.(type) {
		case *ast.ObjectList:
			// If the item is an object list, we need to flatten out the items.
			// Child keys are assumed to be added to the above call to encode
			itemKey := &ast.ObjectKey{Token: tkn}
			for _, obj := range typ.Items {
				keys := append([]*ast.ObjectKey{itemKey}, obj.Keys...)
				l = append(l, &ast.ObjectItem{
					Keys: keys,
					Val:  obj.Val,
				})
			}

		default:
			item := &ast.ObjectItem{
				Keys: []*ast.ObjectKey{{Token: tkn}},
				Val:  val,
			}
			if childKey != nil {
				item.Keys = append(item.Keys, childKey...)
			}
			l = append(l, item)

		}

	}

	sort.Sort(l)
	return &ast.ObjectType{List: &ast.ObjectList{Items: []*ast.ObjectItem(l)}}, nil, nil
}

// encodeStruct converts a struct type into an ast.ObjectType. An ast.ObjectKey
// may be returned if a KeyTag is present that should be used by a parent
// ast.ObjectItem if this node is nested.
func encodeStruct(in reflect.Value) (ast.Node, []*ast.ObjectKey, error) {
	l := in.NumField()
	list := &ast.ObjectList{Items: make([]*ast.ObjectItem, 0, l)}
	keys := make([]*ast.ObjectKey, 0)

	for i := 0; i < l; i++ {
		field := in.Type().Field(i)
		meta := extractFieldMeta(field)

		// these tags are used for debugging the decoder
		// they should not be output
		if meta.unusedKeys || meta.decodedFields || meta.omit {
			continue
		}

		tkn, _ := tokenize(reflect.ValueOf(meta.name), true) // impossible to not be string

		// if the OmitEmptyTag is provided, check if the value is its zero value.
		rawVal := in.Field(i)
		if meta.omitEmpty {
			zeroVal := reflect.Zero(rawVal.Type()).Interface()
			if reflect.DeepEqual(rawVal.Interface(), zeroVal) {
				continue
			}
		}

		val, childKeys, err := encode(rawVal)
		if err != nil {
			return nil, nil, err
		}
		if val == nil {
			continue
		}

		// this field is a key and should be bubbled up to the parent node
		if meta.key {
			if lit, ok := val.(*ast.LiteralType); ok && lit.Token.Type == token.STRING {
				keys = append(keys, &ast.ObjectKey{Token: lit.Token})
				continue
			}
			return nil, nil, errors.New("struct key fields must be string literals")
		}

		// this field is anonymous and should be squashed into the parent struct's fields
		if meta.anonymous && meta.squash {
			switch val := val.(type) {
			case *ast.ObjectType:
				list.Items = append(list.Items, val.List.Items...)
				if childKeys != nil {
					keys = childKeys
				}
				continue
			}
		}

		itemKey := &ast.ObjectKey{Token: tkn}

		// if the item is an object list, we need to flatten out the items
		if objectList, ok := val.(*ast.ObjectList); ok {
			for _, obj := range objectList.Items {
				objectKeys := append([]*ast.ObjectKey{itemKey}, obj.Keys...)
				list.Add(&ast.ObjectItem{
					Keys: objectKeys,
					Val:  obj.Val,
				})
			}
			continue
		}

		item := &ast.ObjectItem{
			Keys: []*ast.ObjectKey{itemKey},
			Val:  val,
		}
		if childKeys != nil {
			item.Keys = append(item.Keys, childKeys...)
		}
		list.Add(item)
	}
	if len(keys) == 0 {
		return &ast.ObjectType{List: list}, nil, nil
	}
	return &ast.ObjectType{List: list}, keys, nil
}

// tokenize converts a primitive type into an token.Token. IDENT tokens (unquoted strings)
// can be optionally triggered for any string types.
func tokenize(in reflect.Value, ident bool) (t token.Token, err error) {
	switch in.Kind() {
	case reflect.Bool:
		return token.Token{
			Type: token.BOOL,
			Text: strconv.FormatBool(in.Bool()),
		}, nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return token.Token{
			Type: token.NUMBER,
			Text: fmt.Sprintf("%d", in.Uint()),
		}, nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return token.Token{
			Type: token.NUMBER,
			Text: fmt.Sprintf("%d", in.Int()),
		}, nil

	case reflect.Float64:
		return token.Token{
			Type: token.FLOAT,
			Text: strconv.FormatFloat(in.Float(), 'g', -1, 64),
		}, nil

	case reflect.String:
		if ident {
			return token.Token{
				Type: token.IDENT,
				Text: in.String(),
			}, nil
		}
		return token.Token{
			Type: token.STRING,
			Text: fmt.Sprintf(`"%s"`, in.String()),
		}, nil
	}

	return t, fmt.Errorf("cannot encode primitive kind %s to token", in.Kind())
}

// extractFieldMeta pulls information about struct fields and the optional HCL tags
func extractFieldMeta(f reflect.StructField) (meta fieldMeta) {
	if f.Anonymous {
		meta.anonymous = true
		meta.name = f.Type.Name()
	} else {
		meta.name = f.Name
	}

	tags := strings.Split(f.Tag.Get(HCLTagName), ",")
	if len(tags) > 0 {
		if tags[0] != "" {
			meta.name = tags[0]
		}

		for _, tag := range tags[1:] {
			switch tag {
			case KeyTag:
				meta.key = true
			case SquashTag:
				meta.squash = true
			case DecodedFieldsTag:
				meta.decodedFields = true
			case UnusedKeysTag:
				meta.unusedKeys = true
			}
		}
	}

	tags = strings.Split(f.Tag.Get(HCLETagName), ",")
	for _, tag := range tags {
		switch tag {
		case OmitTag:
			meta.omit = true
		case OmitEmptyTag:
			meta.omitEmpty = true
		}
	}

	return
}

// deref safely dereferences interface and pointer values to their underlying value types.
// It also detects if that value is invalid or nil.
func deref(in reflect.Value) (val reflect.Value, isNil bool) {
	switch in.Kind() {
	case reflect.Invalid:
		return in, true
	case reflect.Interface, reflect.Ptr:
		if in.IsNil() {
			return in, true
		}
		// recurse for the elusive double pointer
		return deref(in.Elem())
	case reflect.Slice, reflect.Map:
		return in, in.IsNil()
	default:
		return in, false
	}
}

type objectItems []*ast.ObjectItem

func (ol objectItems) Len() int      { return len(ol) }
func (ol objectItems) Swap(i, j int) { ol[i], ol[j] = ol[j], ol[i] }
func (ol objectItems) Less(i, j int) bool {
	iKeys := ol[i].Keys
	jKeys := ol[j].Keys
	for k := 0; k < len(iKeys) && k < len(jKeys); k++ {
		if iKeys[k].Token.Text == jKeys[k].Token.Text {
			continue
		}
		return iKeys[k].Token.Text < jKeys[k].Token.Text
	}
	return len(iKeys) <= len(jKeys)
}
