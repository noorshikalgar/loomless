//go:build linux

package capture

// AXElement mirrors axelement_darwin.go's type — kept identical across
// platforms so callers (internal/engine's auto-caption builder) don't
// need a build-tagged branch of their own just to read the zero value.
type AXElement struct {
	Role  string
	Label string
}

// AXElementAt has no Linux implementation yet — same gap RunClickHook
// already has (see clickhook_linux.go): automatic click-capture itself
// isn't implemented on Linux, so there's never a click to ground an
// element query against in the first place. Always returns ok=false;
// callers already have to handle "no grounding available" as the normal
// case (permission not granted, no element found), so this needs no
// special-casing beyond that.
func AXElementAt(x, y float64) (AXElement, bool) {
	return AXElement{}, false
}
