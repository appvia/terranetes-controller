package utils

import (
	"fmt"

	"github.com/tidwall/pretty"
)

// ByteCountSI returns the number of bytes in the given number of SI units.
func ByteCountSI(b int64) string {
	const unit = 1000

	if b < unit {
		return fmt.Sprintf("%d B", b)
	}

	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "kMGTPE"[exp])
}

// PrettyJSON returns a pretty-printed version of the given JSON data.
func PrettyJSON(data []byte) []byte {
	return pretty.Pretty(data)
}
