package rheos

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// ParFilterMap is like FilterMap, but runs the mapping and filtering operations concurrently with num goroutines.
// The order of the output elements is undefined.
// It's better to use it with a buffered stream.
func ParFilterMap[I any, O any](pipe Stream[I], num int, callback func(context.Context, I) (O, bool, error), ops ...Option[O]) Stream[O] {
	output := make(chan O)
	for _, op := range ops {
		output = op()
	}

	eg, ctx := errgroup.WithContext(pipe.ctx)
	pipe.eg.Go(func() error { // goroutine which spawns more goroutines
		defer close(output)

		for i := 0; i < num; i++ {
			eg.Go(func() error {
				for elem := range pipe.in {
					mapped, ok, err := callback(ctx, elem)
					if err != nil {
						return err
					}
					if !ok {
						continue
					}

					if err := push(ctx, output, mapped); err != nil {
						return err
					}
				}

				return nil
			})
		}

		return eg.Wait()
	})

	return Stream[O]{
		in:  output,
		eg:  pipe.eg,
		ctx: pipe.ctx,
	}
}

// ParMap is like Map, but runs the mapping operations concurrently with num goroutines.
// The order of the output elements is undefined.
// It's better to use it with a buffered stream.
func ParMap[I any, O any](pipe Stream[I], num int, mapper func(context.Context, I) (O, error), ops ...Option[O]) Stream[O] {
	return ParFilterMap[I, O](
		pipe,
		num,
		func(ctx context.Context, elem I) (O, bool, error) {
			mapped, err := mapper(ctx, elem)

			return mapped, true, err
		},
		ops...,
	)
}

// ParFilter is like Filter, but runs the filtering operations concurrently with num goroutines.
// The order of the output elements is undefined.
// It's better to use it with a buffered stream.
func ParFilter[I any](pipe Stream[I], num int, callback func(context.Context, I) (bool, error), ops ...Option[I]) Stream[I] {
	return ParFilterMap[I, I](
		pipe,
		num,
		func(ctx context.Context, elem I) (I, bool, error) {
			ok, err := callback(ctx, elem)

			return elem, ok, err
		},
		ops...,
	)
}
