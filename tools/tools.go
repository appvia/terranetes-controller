//go:build tools
// +build tools

package tools

import (
	_ "github.com/client9/misspell/cmd/misspell"
	_ "github.com/go-bindata/go-bindata/v3/go-bindata"
	_ "github.com/go-swagger/go-swagger/cmd/swagger"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/mikefarah/yq/v3"
	_ "github.com/mitchellh/gox"
	_ "github.com/stretchr/testify/assert"
	_ "github.com/stretchr/testify/require"
	_ "github.com/tcnksm/ghr"
	_ "golang.org/x/tools/cmd/goimports"
	_ "k8s.io/code-generator"
	_ "k8s.io/kube-openapi/cmd/openapi-gen"
	_ "mvdan.cc/sh/v3/cmd/shfmt"
	_ "sigs.k8s.io/controller-runtime/pkg/client/fake"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
