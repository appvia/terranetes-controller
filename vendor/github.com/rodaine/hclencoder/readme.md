# hclencoder<br>[![Build Status](https://travis-ci.org/rodaine/hclencoder.svg?branch=master)](https://travis-ci.org/rodaine/hclencoder) [![GoDoc](https://godoc.org/github.com/rodaine/hclencoder?status.svg)](https://godoc.org/github.com/rodaine/hclencoder)

`hclencoder` encodes/marshals/converts Go types into [HCL (Hashicorp Configuration Language)][HCL]. `hclencoder` ensures correctness in the generated HCL, and can be useful for creating programmatic, type-safe config files.

```go
type Farm struct {
  Name     string    `hcl:"name"`
  Owned    bool      `hcl:"owned"`
  Location []float64 `hcl:"location"`
}

type Farmer struct {
  Name                 string `hcl:"name"`
  Age                  int    `hcl:"age"`
  SocialSecurityNumber string `hcle:"omit"`
}

type Animal struct {
  Name  string `hcl:",key"`
  Sound string `hcl:"says" hcle:"omitempty"`
}

type Config struct {
  Farm      `hcl:",squash"`
  Farmer    Farmer            `hcl:"farmer"`
  Animals   []Animal          `hcl:"animal"`
  Buildings map[string]string `hcl:"buildings"`
}

input := Config{
  Farm: Farm{
    Name:     "Ol' McDonald's Farm",
    Owned:    true,
    Location: []float64{12.34, -5.67},
  },
  Farmer: Farmer{
    Name:                 "Robert Beauregard-Michele McDonald, III",
    Age:                  65,
    SocialSecurityNumber: "please-dont-share-me",
  },
  Animals: []Animal{
    {
      Name:  "cow",
      Sound: "moo",
    },
    {
      Name:  "pig",
      Sound: "oink",
    },
    {
      Name: "rock",
    },
  },
  Buildings: map[string]string{
    "House": "123 Numbers Lane",
    "Barn":  "456 Digits Drive",
  },
}

hcl, err := Encode(input)
if err != nil {
  log.Fatal("unable to encode: ", err)
}

fmt.Print(string(hcl))

// Output:
// name = "Ol' McDonald's Farm"
//
// owned = true
//
// location = [
//   12.34,
//   -5.67,
// ]
//
// farmer {
//   name = "Robert Beauregard-Michele McDonald, III"
//   age  = 65
// }
//
// animal "cow" {
//   says = "moo"
// }
//
// animal "pig" {
//   says = "oink"
// }
//
// animal "rock" {}
//
// buildings {
//   Barn  = "456 Digits Drive"
//   House = "123 Numbers Lane"
// }
//
```

## Features

- [x] Encodes any `struct` or `map[string]T` type as the input for the generated HCL
- [x] Supports all value, interface, and pointer types supported by the HCL encoder: `bool`, `int`, `float64`, `string`, `struct`, `[]T`, `map[string]T`
- [x] Uses the [HCL Printer][hclprinter] to ensure consistency with the output HCL
- [x] Map types are sorted to ensure ordering
- [ ] Support raw HCL [`ast.Node`][node] types in the struct.
- [ ] Support `HCLMarshaler` interface for types to encode themselves, similar to [`json.Marshaler`][jsonmarshal]


## Struct Tags

`hclencoder` supports and respects the existing `hcl` [struct tags][tags]:

- **`hcl:"custom_name"`** - specifies the name of the field as represented in the output HCL to be `custom_name`. The default behavior is to use the unmodified name of the field. If other tag fields are desired but the default name behavior should be used, leave the first comma-delimited value empty (eg, `hcl:",key"`).

- **`hcl:",key"`** - indicates the field should be used as part of the compound key for the HCL block. This field must be of type `string`.

- **`hcl:",squash"`** - attached to anonymous fields of a struct, indicates to lift the fields of that value into the parent block's scope transparently. Otherwise, the field's type is used as the key for the value.

- **`hcl:",unusedKeys"`** - identifies this debug field which stores any unused keys found by the decoder. This field shoudl be of type `[]string`. This has the same behavior as the `hcle:"omit"` tag and is not encoded.

- **`hcl:",decodedFields"`** - identifies this debug field which stores the names of all fields decoded from HCL. This field should be of type `[]string`. This has the same behavior as the `hcle:"omit"` tag and is not encoded.

`hclencoder` also supports additional `hcle` struct tags that provide additional capabilities:

- **`hcle:"omit"`** - omits this field from encoding into HCL. This is similar behavior to [`json:"-"`][json].

- **`hcle:"omitempty"`** - omits this field if it is a zero value for its type. This is similar behavior to [`json:",omitempty"`][json].

[HCL]:         https://github.com/hashicorp/hcl
[hclprinter]:  https://godoc.org/github.com/hashicorp/hcl/hcl/printer
[json]:        https://golang.org/pkg/encoding/json/#Marshal
[jsonmarshal]: https://golang.org/pkg/encoding/json/#Marshaler
[node]:        https://godoc.org/github.com/hashicorp/hcl/hcl/ast#Node
[tags]:        https://golang.org/pkg/reflect/#StructTag

## License

The MIT License (MIT)

Copyright (c) 2016 Chris Roche

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
