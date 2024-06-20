package rheos_test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/dmksnnk/rheos"
)

func ExampleReduce() {
	// count the number of elements in the stream
	ctx := context.Background()
	producer := rheos.FromSlice(ctx, []int{1, 2, 3, 4, 5})
	got, err := rheos.Reduce(
		producer,
		func(acc int, _ int) (int, error) {
			return acc + 1, nil
		},
		0,
	)
	fmt.Println(got, err)
	// Output: 5 <nil>
}

func ExampleMap() {
	// maps integers to strings
	ctx := context.Background()
	producer := rheos.FromSlice(ctx, []int{1, 2, 3, 4, 5})
	strings := rheos.Map(producer, func(_ context.Context, v int) (string, error) {
		return strconv.Itoa(v), nil
	})
	got, err := rheos.Collect(strings)
	fmt.Printf("%#v, %#v\n", got, err)
	// Output: []string{"1", "2", "3", "4", "5"}, <nil>
}

func ExampleFilter() {
	// filter out odd numbers
	producer := rheos.FromSlice(context.Background(), []int{1, 2, 3, 4, 5})
	strings := rheos.Filter(producer, func(_ context.Context, v int) (bool, error) {
		return v%2 == 0, nil
	})
	got, err := rheos.Collect(strings)
	fmt.Println(got, err)
	// Output: [2 4] <nil>
}

func ExampleBatchTimeout() {
	producer := rheos.FromSlice(context.Background(), []int{1, 2, 3, 4, 5})
	workSimulation := rheos.Map(producer, func(_ context.Context, v int) (int, error) {
		time.Sleep(2 * time.Millisecond) // simulate work which is longer than the batch timeout
		return v, nil
	})
	batch := rheos.BatchTimeout(workSimulation, 2, time.Millisecond)
	got, err := rheos.Collect(batch)
	fmt.Println(got, err) // instead of batches of 2, we get batches of 1 because the work takes longer than the batch timeout
	// Output: [[1] [2] [3] [4] [5]] <nil>
}
