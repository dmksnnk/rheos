package rheos_test

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dmksnnk/rheos"
)

func TestParallel(t *testing.T) {
	want := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}

	start := time.Now()
	prod := newProducer(context.TODO(), 10)
	strings := rheos.ParMap(prod, 10, func(ctx context.Context, i int) (string, error) {
		time.Sleep(100 * time.Millisecond)
		return strconv.Itoa(i), nil
	})
	got, err := rheos.Collect(strings)
	if err != nil {
		t.Fatal(err)
	}

	sort.Slice(got, func(i, j int) bool {
		return got[i] < got[j]
	})
	assertSlicesEqual(t, want, got)

	elapsed := time.Since(start)
	if elapsed > 200*time.Millisecond {
		t.Errorf("elapsed time %s, want less than 200ms", elapsed)
	}
}

func TestParallelPipeline(t *testing.T) {
	testFn := func(producer rheos.Stream[int], mapFn func(context.Context, int) (int, error), filterMapFn func(context.Context, int) (int, bool, error)) ([]int, error) {
		size := rand.Intn(10) + 1
		p2 := rheos.ParMap(producer, size, mapFn)
		p3 := rheos.ParFilterMap(p2, size, filterMapFn)
		p4 := rheos.Batch(p3, rand.Intn(10)+1)
		p5 := rheos.UnBatch(p4)

		return rheos.Collect(p5)
	}
	noopMapFn := func(_ context.Context, v int) (int, error) {
		return v, nil
	}
	noopFilterMapFn := func(_ context.Context, v int) (int, bool, error) {
		return v, true, nil
	}

	t.Run("passes", func(t *testing.T) {
		num := int(rand.Int31n(10) + 2)
		want := intRange(num)

		got, err := testFn(newProducer(context.TODO(), num), noopMapFn, noopFilterMapFn)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		sort.Slice(got, func(i, j int) bool {
			return got[i] < got[j]
		})
		assertSlicesEqual(t, want, got)
	})

	t.Run("pass cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		num := int(rand.Int31n(10) + 2)

		_, err := testFn(newProducer(ctx, num), noopMapFn, noopFilterMapFn)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("unexpected error: %v, want: %v", err, context.Canceled)
		}
	})

	t.Run("context is cancelled", func(t *testing.T) {
		num := int(rand.Int31n(10) + 2)

		i := int32(0)
		ctx, cancel := context.WithCancel(context.Background())
		cancellingMapFn := func(ctx context.Context, v int) (int, error) {
			j := atomic.AddInt32(&i, 1)
			if int(j) >= num/2 {
				cancel()
			}
			return v, nil
		}

		_, err := testFn(newProducer(ctx, num), cancellingMapFn, noopFilterMapFn)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("unexpected error: %v, want: %v", err, context.Canceled)
		}
	})

	t.Run("producer error", func(t *testing.T) {
		num := int(rand.Int31n(10) + 2)
		prod := rheos.FromIter(context.TODO(), func(yield func(v int) bool) error {
			for i := 0; i < num; i++ {
				if i >= num/2 {
					return errTest
				}

				if !yield(i) {
					break
				}
			}

			return nil
		})

		_, err := testFn(prod, noopMapFn, noopFilterMapFn)
		if !errors.Is(err, errTest) {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("step error", func(t *testing.T) {
		num := int(rand.Int31n(10) + 2)

		i := int32(0)
		errMapFn := func(ctx context.Context, v int) (int, error) {
			j := atomic.AddInt32(&i, 1)
			if int(j) >= num/2 {
				return v, errTest
			}

			return v, nil
		}

		_, err := testFn(newProducer(context.TODO(), num), errMapFn, noopFilterMapFn)
		if !errors.Is(err, errTest) {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
