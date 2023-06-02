package utils

import (
	"fmt"
	"go/build"
	"os"
	"path"
)

type Expander interface {
	Expand() ([]string, error)
}

type ExpanderMap map[string]Expander

var (
	PathExpandable = ExpanderMap{
		"$all":  &allExpander{},
		"$test": &testExpander{},
	}
	PackageExpandable = ExpanderMap{
		"$gostd": &gostdExpander{},
	}
)

type allExpander struct{}

func (*allExpander) Expand() ([]string, error) {
	return []string{"**/*.go"}, nil
}

type testExpander struct{}

func (*testExpander) Expand() ([]string, error) {
	return []string{"**/*_test.go"}, nil
}

type gostdExpander struct {
	cache []string
}

// We can do this as all imports that are not root are either prefixed with a domain
// or prefixed with `./` or `/` to dictate it is a local file reference
func (e *gostdExpander) Expand() ([]string, error) {
	if len(e.cache) != 0 {
		return e.cache, nil
	}
	root := path.Join(build.Default.GOROOT, "src")
	fs, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("could not read GOROOT directory: %w", err)
	}
	var pkgPrefix []string
	for _, f := range fs {
		if !f.IsDir() {
			continue
		}
		pkgPrefix = append(pkgPrefix, f.Name())
	}
	e.cache = pkgPrefix
	return pkgPrefix, nil
}

func ExpandSlice(sl []string, exp ExpanderMap) ([]string, error) {
	for i, s := range sl {
		f, found := exp[s]
		if !found {
			continue
		}
		e, err := f.Expand()
		if err != nil {
			return nil, fmt.Errorf("couldn't expand %s: %w", s, err)
		}
		sl = insertSlice(sl, i, e...)
	}
	return sl, nil
}

func ExpandMap(m map[string]string, exp ExpanderMap) error {
	for k, v := range m {
		f, found := exp[k]
		if !found {
			continue
		}
		e, err := f.Expand()
		if err != nil {
			return fmt.Errorf("couldn't expand %s: %w", k, err)
		}
		for _, ex := range e {
			m[ex] = v
		}
		delete(m, k)
	}
	return nil
}

func insertSlice(a []string, k int, b ...string) []string {
	n := len(a) + len(b) - 1
	if n <= cap(a) {
		a2 := a[:n]
		copy(a2[k+len(b):], a[k+1:])
		copy(a2[k:], b)
		return a2
	}
	a2 := make([]string, n)
	copy(a2, a[:k])
	copy(a2[k:], b)
	copy(a2[k+len(b):], a[k+1:])
	return a2
}
