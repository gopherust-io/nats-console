package apikit

// NonNilSlice kept for call-site compatibility. Nil slices marshal as []
// via serializer's sonic NoNullSliceOrMap config.
func NonNilSlice[T any](s []T) []T {
	return s
}
