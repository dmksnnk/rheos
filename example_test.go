package rheos_test

import (
	"context"
	"fmt"
	"strconv"

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
