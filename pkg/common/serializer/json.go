package serializer

import (
	"github.com/bytedance/sonic"
)

// NoNullSliceOrMap encodes nil slices/maps as []/{} so list handlers
// need not allocate empty literals via nonNilSlice.
var sonicDefaultCfg = sonic.Config{
	NoNullSliceOrMap: true,
}.Froze()

func Marshal(v any) ([]byte, error) {
	return sonicDefaultCfg.Marshal(v)
}

func Unmarshal(body []byte, v any) error {
	return sonicDefaultCfg.Unmarshal(body, v)
}
