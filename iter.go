//go:build go1.23

package rheos

import (
	"context"
	"iter"

	"golang.org/x/sync/errgroup"
)

// FromSeq2 converts iterator with value-error pair to a Stream.
// If seq returns error or context is cancelled during processing,
// Stream stops processing and returns error.
func FromSeq2[I any](ctx context.Context, seq iter.Seq2[I, error], ops ...Option[I]) Stream[I] {
	results := make(chan I)
	for _, op := range ops {
		results = op()
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		defer close(results)

		var err error
		seq(func(elem I, seqErr error) bool {
			err = seqErr
			if seqErr != nil {
				return false
			}

			err = push(ctx, results, elem)
			return err == nil
		})

		return err
	})

	return Stream[I]{
		in:  results,
		eg:  eg,
		ctx: ctx,
	}
}

// FromSeq converts value iterator to a Stream.
// If context is cancelled during processing, Stream stops processing and returns error.
func FromSeq[I any](ctx context.Context, seq iter.Seq[I], ops ...Option[I]) Stream[I] {
	return FromSeq2[I](
		ctx,
		func(yield func(I, error) bool) {
			seq(func(elem I) bool {
				return yield(elem, nil)
			})
		},
		ops...,
	)
}

// All returns an iterator over value-error pairs.
func All[I any](pipe Stream[I]) iter.Seq2[I, error] {
	return func(yield func(I, error) bool) {
		for elem := range pipe.in {
			if err := pipe.ctx.Err(); err != nil {
				yield(elem, err)
				return
			}

			if !yield(elem, nil) {
				return
			}
		}
	}
}
