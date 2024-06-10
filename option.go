package rheos

// Option to configure the pipeline steps.
type Option[T any] func() chan T

// WithBuffer sets the stream buffer capacity.
func WithBuffer[T any](size int) Option[T] {
	return func() chan T {
		return make(chan T, size)
	}
}
