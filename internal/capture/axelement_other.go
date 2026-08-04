//go:build !darwin && !linux

package capture

// AXElement mirrors axelement_darwin.go's type — see that file's doc
// comment for field meanings.
type AXElement struct {
	Role  string
	Label string
}

// AXElementAt has no implementation on this platform — see
// clickhook_other.go, which is the reason there's never a click to
// ground an element query against here either.
func AXElementAt(x, y float64) (AXElement, bool) {
	return AXElement{}, false
}
