package strings

import (
	"unsafe"
)

func StringToBytes(s string) []byte {
	if IsEmpty(s) {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s)) //nolint:gosec // G103: zero-copy string view
}

func BytesToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b), len(b)) //nolint:gosec // G103: zero-copy bytes view
}

func IsEmpty(s string) bool {
	return len(s) == 0
}
