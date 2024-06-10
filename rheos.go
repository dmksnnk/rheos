// Package rheos provides building blocks for a stream processing.
package rheos

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// Stream is a base element of data steam processing pipeline.
type Stream[I any] struct {
	in  <-chan I
	eg  *errgroup.Group
	ctx context.Context
}

// Seq is an iterator over sequences of individual values.
// When called as seq(ctx, yield), seq calls yield(v) for each value v in the sequence,
// stopping early if yield returns false (works as break) or error occurred.
// Based on https://go.dev/wiki/RangefuncExperiment.
type Seq[T any] func(yield func(T) bool) error

// FromIter creates a new Stream from a Seq.
// If seq returns error or context is cancelled during processing,
// Stream stops processing and returns error.
func FromIter[I any](ctx context.Context, seq Seq[I], ops ...Option[I]) Stream[I] {
	results := make(chan I)
	for _, op := range ops {
		results = op()
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		defer close(results)

		var err error
		pushFn := func(elem I) bool {
			err = push(ctx, results, elem)
			return err == nil
		}

		if err := seq(pushFn); err != nil {
			return err
		}

		return err
	})

	return Stream[I]{
		in:  results,
		eg:  eg,
		ctx: ctx,
	}
}

// FromSlice creates a new Stream from a slice.
// If seq returns error or context is cancelled during processing,
// Stream stops processing and returns error.
func FromSlice[I any](ctx context.Context, slice []I, ops ...Option[I]) Stream[I] {
	seq := func(yield func(I) bool) error {
		for _, elem := range slice {
			if !yield(elem) {
				break
			}
		}

		return nil
	}

	return FromIter[I](ctx, seq, ops...)
}

// Map transforms Stream into a Stream of another type.
// If error occurs or context is cancelled during processing, Map stops processing and returns error.
func Map[I any, O any](pipe Stream[I], mapper func(context.Context, I) (O, error), ops ...Option[O]) Stream[O] {
	output := make(chan O)
	for _, op := range ops {
		output = op()
	}

	pipe.eg.Go(func() error {
		defer close(output)

		for elem := range pipe.in {
			mapped, err := mapper(pipe.ctx, elem)
			if err != nil {
				return err
			}

			if err := push(pipe.ctx, output, mapped); err != nil {
				return err
			}
		}

		return nil
	})

	return Stream[O]{
		in:  output,
		eg:  pipe.eg,
		ctx: pipe.ctx,
	}
}

// Filter returns a Stream which obtained after filtering using given callback function.
// The callback function should return  whether the element should be included or not.
// If error occurs or context is cancelled during processing, Filter stops processing and returns error.
func Filter[I any](pipe Stream[I], callback func(context.Context, I) (bool, error), ops ...Option[I]) Stream[I] {
	return FilterMap[I, I](
		pipe,
		func(ctx context.Context, elem I) (I, bool, error) {
			ok, err := callback(ctx, elem)

			return elem, ok, err
		},
		ops...,
	)
}

// FilterMap returns a Stream which obtained after both filtering and mapping using the given callback function.
// The callback function should return result of the mapping operation and whether the element should be included or not.
// If error occurs or context is cancelled during processing, FilterMap stops processing and returns error.
func FilterMap[I any, O any](pipe Stream[I], callback func(context.Context, I) (O, bool, error), ops ...Option[O]) Stream[O] {
	output := make(chan O)
	for _, op := range ops {
		output = op()
	}

	pipe.eg.Go(func() error {
		defer close(output)

		for elem := range pipe.in {
			mapped, ok, err := callback(pipe.ctx, elem)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}

			if err := push(pipe.ctx, output, mapped); err != nil {
				return err
			}
		}

		return nil
	})

	return Stream[O]{
		in:  output,
		eg:  pipe.eg,
		ctx: pipe.ctx,
	}
}

// Batch converts a steam of elements into a steam of slices of elements of given size.
// If context is cancelled during processing, Batch stops processing and returns error.
func Batch[I any](pipe Stream[I], size int, ops ...Option[[]I]) Stream[[]I] {
	output := make(chan []I)
	for _, op := range ops {
		output = op()
	}

	pipe.eg.Go(func() error {
		defer close(output)

		batch := make([]I, 0, size)
		for elem := range pipe.in {
			batch = append(batch, elem)
			if len(batch) == size {
				if err := push(pipe.ctx, output, batch); err != nil {
					return err
				}

				batch = make([]I, 0, size)
			}
		}

		if len(batch) > 0 {
			return push(pipe.ctx, output, batch)
		}

		return nil
	})

	return Stream[[]I]{
		in:  output,
		eg:  pipe.eg,
		ctx: pipe.ctx,
	}
}

// UnBatch converts a stream of slices of elements into a stream of elements.
// If context is cancelled during processing, UnBatch stops processing and returns error.
func UnBatch[I any](pipe Stream[[]I], ops ...Option[I]) Stream[I] {
	output := make(chan I)
	for _, op := range ops {
		output = op()
	}

	pipe.eg.Go(func() error {
		defer close(output)

		for batch := range pipe.in {
			for _, elem := range batch {
				if err := push(pipe.ctx, output, elem); err != nil {
					return err
				}
			}
		}

		return nil
	})

	return Stream[I]{
		in:  output,
		eg:  pipe.eg,
		ctx: pipe.ctx,
	}
}

// ForEach processes each element in the stream using the given callback function.
// If callback returns error or context is cancelled during processing, ForEach stops and returns error.
func ForEach[I any](pipe Stream[I], callback func(context.Context, I) error) error {
	pipe.eg.Go(func() error {
		for elem := range pipe.in {
			if pipe.ctx.Err() != nil {
				return pipe.ctx.Err()
			}

			if err := callback(pipe.ctx, elem); err != nil {
				return err
			}
		}

		return nil
	})

	return pipe.eg.Wait()
}

// Reduce reduces a stream to a value which is the accumulated result of running each element in collection
// through accumulator, where each successive invocation is supplied the return value of the previous.
// If accum returns error or context is cancelled during processing, Reduce stops and returns error.
//
//nolint:ireturn // ireturn suggests to return `any`, but we need to return specific type
func Reduce[I any, R any](pipe Stream[I], accum func(R, I) (R, error), initial R) (R, error) {
	fn := func(ctx context.Context, elem I) (err error) {
		initial, err = accum(initial, elem) // a little bit of magical, but it works

		return
	}

	err := ForEach(pipe, fn)

	return initial, err
}

// Collect collects all elements from the stream into a slice.
// If context is cancelled during processing, Collect stops and returns error.
func Collect[I any](p Stream[I]) ([]I, error) {
	return Reduce(
		p,
		func(acc []I, v I) ([]I, error) {
			return append(acc, v), nil
		},
		[]I{},
	)
}

func push[T any](ctx context.Context, ch chan<- T, item T) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case ch <- item:
		return nil
	}
}
