//go:build go1.23

package rheos_test

import (
	"context"
	"errors"
	"iter"
	"maps"
	"math/rand"
	"slices"
	"testing"

	"github.com/dmksnnk/rheos"
)

func TestFromSeq2(t *testing.T) {
	t.Run("no error", func(t *testing.T) {
		num := rand.Intn(10) + 1
		want := intRange(num)

		p1 := rheos.FromSeq2(context.TODO(), seq(num))
		got, err := rheos.Collect(p1)
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if !slices.Equal(want, got) {
			t.Errorf("want %v, got %v", want, got)
		}
	})

	t.Run("with error", func(t *testing.T) {
		vals := map[int]error{1: nil, 2: nil, 3: errTest, 4: nil, 5: nil}

		p1 := rheos.FromSeq2(context.TODO(), maps.All(vals))
		_, err := rheos.Collect(p1)
		if !errors.Is(err, errTest) {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		num := rand.Intn(10) + 1
		ctx, cancel := context.WithCancel(context.Background())

		s := rheos.FromSeq2(ctx, seq(num))
		i := 0
		p2 := rheos.Map(s, func(ctx context.Context, v int) (int, error) {
			i++
			if i >= num/2 {
				cancel()
			}

			return v, nil
		})
		_, err := rheos.Collect(p2)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("unexpected error: %s", err)
		}
	})
}

func TestFromSeq(t *testing.T) {
	t.Run("no error", func(t *testing.T) {
		vals := intRange(rand.Intn(10) + 1)

		s := rheos.FromSeq(context.TODO(), slices.Values(vals))
		got, err := rheos.Collect(s)
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if !slices.Equal(vals, got) {
			t.Errorf("want %v, got %v", vals, got)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		vals := []int{1, 2, 3, 4, 5}
		ctx, cancel := context.WithCancel(context.Background())

		p1 := rheos.FromSeq(ctx, slices.Values(vals))
		i := 0
		p2 := rheos.Map(p1, func(ctx context.Context, v int) (int, error) {
			i++
			if i >= len(vals)/2 {
				cancel()
			}

			return v, nil
		})
		_, err := rheos.Collect(p2)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("unexpected error: %s", err)
		}
	})
}

func TestAll(t *testing.T) {
	t.Run("no error", func(t *testing.T) {
		vals := []int{1, 2, 3, 4, 5}
		want := map[int]error{1: nil, 2: nil, 3: nil, 4: nil, 5: nil}

		s := rheos.FromSeq(context.TODO(), slices.Values(vals))
		got := maps.Collect(rheos.All(s))
		if !maps.Equal(want, got) {
			t.Errorf("want %v, got %v", want, got)
		}
	})

	t.Run("stop early", func(t *testing.T) {
		vals := []int{1, 2, 3, 4, 5}
		s := rheos.FromSeq(context.TODO(), slices.Values(vals))
		want := map[int]error{1: nil, 2: nil}

		collected := make(map[int]error)
		for i, err := range rheos.All(s) {
			collected[i] = err
			if i == 2 {
				break
			}
		}

		if !maps.Equal(want, collected) {
			t.Errorf("want %v, got %v", want, collected)
		}
	})
}

func seq(n int) iter.Seq2[int, error] {
	return func(yield func(int, error) bool) {
		for i := 0; i < n; i++ {
			if !yield(i, nil) {
				return
			}
		}
	}
}
