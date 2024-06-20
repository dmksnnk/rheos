package rheos_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/dmksnnk/rheos"
	"golang.org/x/sync/errgroup"
)

func TestUnitPipeline(t *testing.T) {
	testFn := func(producer rheos.Stream[int], mapFn func(context.Context, int) (int, error), filterMapFn func(context.Context, int) (int, bool, error)) ([]int, error) {
		p2 := rheos.Map(producer, mapFn)
		p3 := rheos.FilterMap(p2, filterMapFn)
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
		got, err := testFn(newProducer(context.TODO(), num), noopMapFn, noopFilterMapFn)

		want := intRange(num)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assertSlicesEqual(t, want, got)
	})

	t.Run("pass cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		num := int(rand.Int31n(10) + 2)

		_, err := testFn(newProducer(ctx, num), noopMapFn, noopFilterMapFn)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("context is cancelled", func(t *testing.T) {
		num := int(rand.Int31n(10) + 2)

		i := 0
		ctx, cancel := context.WithCancel(context.Background())
		cancellingMapFn := func(ctx context.Context, v int) (int, error) {
			i++
			if i >= num/2 {
				cancel()
			}
			return v, nil
		}

		_, err := testFn(newProducer(ctx, num), cancellingMapFn, noopFilterMapFn)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("unexpected error: %v", err)
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

		i := 0
		errMapFn := func(ctx context.Context, v int) (int, error) {
			i++
			if i >= num/2 {
				return i, errTest
			}
			return v, nil
		}

		_, err := testFn(newProducer(context.TODO(), num), errMapFn, noopFilterMapFn)
		if !errors.Is(err, errTest) {
			t.Errorf("unexpected error: %v, want: %v", err, errTest)
		}
	})
}

func TestUnitForEach(t *testing.T) {
	t.Run("collect items", func(t *testing.T) {
		num := int(rand.Int31n(100) + 10)
		p := newProducer(context.Background(), num)

		var result []int
		err := rheos.ForEach(
			p,
			func(_ context.Context, v int) error {
				result = append(result, v)
				return nil
			},
		)

		want := intRange(num)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertSlicesEqual(t, want, result)
	})
	t.Run("returns error", func(t *testing.T) {
		num := int(rand.Int31n(100) + 10)
		p := newProducer(context.Background(), num)

		var result []int
		err := rheos.ForEach(
			p,
			func(_ context.Context, v int) error {
				result = append(result, v)
				if len(result) >= num/2 {
					return errTest
				}
				return nil
			},
		)

		if !errors.Is(err, errTest) {
			t.Errorf("unexpected error: %v, want: %v", err, errTest)
		}
	})
}

func TestUnitReduce(t *testing.T) {
	t.Run("count items", func(t *testing.T) {
		num := int(rand.Int31n(100))
		p := newProducer(context.Background(), num)
		got, err := rheos.Reduce(
			p,
			func(acc int, _ int) (int, error) {
				return acc + 1, nil
			},
			0,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != num {
			t.Errorf("should count all items: %d != %d", num, got)
		}
	})
	t.Run("returns error", func(t *testing.T) {
		num := int(rand.Int31n(100) + 10)
		p := newProducer(context.Background(), num)
		_, err := rheos.Reduce(
			p,
			func(acc int, _ int) (int, error) {
				if acc >= num/2 {
					return acc, errTest
				}
				return acc + 1, nil
			},
			0,
		)

		if !errors.Is(err, errTest) {
			t.Errorf("unexpected error: %v, want: %v", err, errTest)
		}
	})
}

func TestUnitBuffered(t *testing.T) {
	order := make(chan string)
	num := 5

	var eg errgroup.Group
	eg.Go(func() error {
		defer close(order)

		p := newProducer(context.Background(), num)
		buf := rheos.Filter(
			p,
			func(ctx context.Context, i int) (bool, error) {
				order <- "buffered"
				return true, nil
			},
			rheos.WithBuffer[int](num),
		)
		unbuf := rheos.Filter(
			buf,
			func(ctx context.Context, i int) (bool, error) {
				time.Sleep(10 * time.Millisecond) // simulate work
				order <- "unbuffered"
				return true, nil
			},
		)
		return rheos.ForEach(unbuf, func(_ context.Context, i int) error {
			return nil
		})
	})

	// fast step with buffer can quickly fill the buffer
	// and then slow step without buffer will start processing
	wantResult := []string{
		"buffered",
		"buffered",
		"buffered",
		"buffered",
		"buffered",
		"unbuffered",
		"unbuffered",
		"unbuffered",
		"unbuffered",
		"unbuffered",
	}

	var result []string
	for i := range order {
		result = append(result, i)
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSlicesEqual(t, wantResult, result)
}

func TestUnitFromChannel(t *testing.T) {
	t.Run("collect items", func(t *testing.T) {
		num := int(rand.Int31n(100) + 10)
		input := make(chan int, num)
		go func() {
			defer close(input)
			for i := 0; i < num; i++ {
				input <- i
			}
		}()

		p := rheos.FromChannel(context.Background(), input)
		got, err := rheos.Collect(p)

		want := intRange(num)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertSlicesEqual(t, want, got)
	})

	t.Run("context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		num := int(rand.Int31n(100) + 10)
		input := make(chan int, num)
		go func() {
			defer close(input)
			for i := 0; i < num; i++ {
				if i >= num/2 {
					cancel()
					return
				}

				input <- i
			}
		}()

		p := rheos.FromChannel(ctx, input)
		_, err := rheos.Collect(p)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("unexpected error: %v, want: %v", err, context.Canceled)
		}
	})
}

func newProducer(ctx context.Context, num int) rheos.Stream[int] {
	return rheos.FromIter(ctx, func(yield func(v int) bool) error {
		for i := 0; i < num; i++ {
			if !yield(i) {
				break
			}
		}

		return nil
	})
}

var errTest = errors.New("test error")

func intRange(length int) []int {
	result := make([]int, length)
	for i := 0; i < length; i++ {
		result[i] = i
	}
	return result
}

func assertSlicesEqual[T comparable](t *testing.T, expected, actual []T) {
	if len(expected) != len(actual) {
		t.Errorf("slices have different lengths: %d != %d", len(expected), len(actual))
		return
	}

	for i, v := range expected {
		if actual[i] != v {
			t.Errorf("slices differ at index %d: %v != %v", i, v, actual[i])
			return
		}
	}
}
