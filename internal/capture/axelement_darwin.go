//go:build darwin

package capture

/*
#cgo LDFLAGS: -framework ApplicationServices -framework CoreFoundation
#include <ApplicationServices/ApplicationServices.h>
#include <stdlib.h>

// cfStringToChar copies a CFStringRef's contents into a newly malloc'd
// C string the Go side is responsible for freeing — CFStringGetCString
// needs a caller-owned buffer, there's no "give me a char* directly" API.
static char *cfStringToChar(CFStringRef str) {
    if (str == NULL) {
        return NULL;
    }
    CFIndex length = CFStringGetLength(str);
    CFIndex maxSize = CFStringGetMaximumSizeForEncoding(length, kCFStringEncodingUTF8) + 1;
    char *buffer = (char *)malloc(maxSize);
    if (buffer != NULL && CFStringGetCString(str, buffer, maxSize, kCFStringEncodingUTF8)) {
        return buffer;
    }
    free(buffer);
    return NULL;
}

// firstNonEmptyStringAttribute tries each attribute in priority order and
// returns the first one that resolves to a non-empty CFString — a single
// element rarely has all of kAXTitleAttribute/kAXDescriptionAttribute/
// kAXValueAttribute populated at once, and which one actually carries the
// human-meaningful label varies by control type (a button usually has a
// title; a text field's own "label" is usually its typed value; a lot of
// icon-only controls only have a description).
static CFStringRef firstNonEmptyStringAttribute(AXUIElementRef element, CFStringRef *attrs, int count) {
    for (int i = 0; i < count; i++) {
        CFTypeRef value = NULL;
        if (AXUIElementCopyAttributeValue(element, attrs[i], &value) == kAXErrorSuccess && value != NULL) {
            if (CFGetTypeID(value) == CFStringGetTypeID() && CFStringGetLength((CFStringRef)value) > 0) {
                return (CFStringRef)value;
            }
            CFRelease(value);
        }
    }
    return NULL;
}

typedef struct {
    char *role;
    char *label;
    int found;
} storyAXResult;

// storyElementAt queries the system-wide accessibility tree for whatever
// UI element sits at (x, y) in screen-point coordinates (the same space
// the click event tap delivers — see clickhook_darwin.go) and pulls out
// a human-readable role plus the best available label. Requires the same
// Accessibility permission the click tap already needs (no new
// permission *category*, but macOS treats "observe global clicks" and
// "inspect element attributes" as separate grants a user can enable
// independently, so this can legitimately fail even when click capture
// itself is working).
static storyAXResult storyElementAt(float x, float y) {
    storyAXResult result = {NULL, NULL, 0};

    AXUIElementRef systemWide = AXUIElementCreateSystemWide();
    AXUIElementRef element = NULL;
    AXError err = AXUIElementCopyElementAtPosition(systemWide, x, y, &element);
    if (err != kAXErrorSuccess || element == NULL) {
        CFRelease(systemWide);
        return result;
    }

    CFTypeRef roleValue = NULL;
    // kAXRoleDescriptionAttribute ("button", "text field", "menu item")
    // is already the human-readable form macOS itself uses for VoiceOver
    // — kAXRoleAttribute alone would give the raw "AXButton"/"AXTextField"
    // constant name instead, which reads wrong in a caption sentence.
    if (AXUIElementCopyAttributeValue(element, kAXRoleDescriptionAttribute, &roleValue) == kAXErrorSuccess && roleValue != NULL) {
        if (CFGetTypeID(roleValue) == CFStringGetTypeID()) {
            result.role = cfStringToChar((CFStringRef)roleValue);
        }
        CFRelease(roleValue);
    }

    CFStringRef labelAttrs[3] = {kAXTitleAttribute, kAXDescriptionAttribute, kAXValueAttribute};
    CFStringRef labelValue = firstNonEmptyStringAttribute(element, labelAttrs, 3);
    if (labelValue != NULL) {
        result.label = cfStringToChar(labelValue);
        CFRelease(labelValue);
    }

    result.found = 1;
    CFRelease(element);
    CFRelease(systemWide);
    return result;
}
*/
import "C"

import "unsafe"

// AXElement describes the UI control found at a screen point, in terms
// meant to read naturally inside a caption sentence — see
// internal/engine's auto-caption builder, the only current consumer.
type AXElement struct {
	// Role is the human-readable control kind ("button", "text field",
	// "menu item", ...), or "" if the accessibility tree didn't offer
	// one for this element.
	Role string
	// Label is the best available title/description/value text for the
	// element, or "" if none of those were populated (common for
	// unlabeled icon-only controls).
	Label string
}

// AXElementAt queries the macOS Accessibility API for the UI element at
// the given screen point (screen-point coordinates, matching what
// RunClickHook's callback delivers) — ok is false if accessibility
// permission isn't granted, no element exists there, or the point is
// off-screen; callers must treat that as "no grounding available," not
// an error worth surfacing to the user (the click was still captured
// either way).
func AXElementAt(x, y float64) (AXElement, bool) {
	res := C.storyElementAt(C.float(x), C.float(y))
	if res.found == 0 {
		return AXElement{}, false
	}
	elem := AXElement{}
	if res.role != nil {
		elem.Role = C.GoString(res.role)
		C.free(unsafe.Pointer(res.role))
	}
	if res.label != nil {
		elem.Label = C.GoString(res.label)
		C.free(unsafe.Pointer(res.label))
	}
	return elem, true
}
