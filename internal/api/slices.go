package api

// nonNilSlice kept for call-site compatibility. Nil slices marshal as []
// via serializer's sonic NoNullSliceOrMap config.
func nonNilSlice[T any](s []T) []T {
	return s
}
