//go:build go1.23

package rheos_test

import (
	"context"
	"fmt"

	"github.com/dmksnnk/rheos"
)

func ExampleFromSeq2() {
	// iter.Seq2[int, error]
	values := func(yield func(int, error) bool) {
		for _, v := range []int{1, 2, 3, 4, 5} {
			if !yield(v, nil) {
				return
			}
		}
	}

	producer := rheos.FromSeq2(context.TODO(), values)
	fmt.Println(rheos.Collect(producer))
	// Output: [1 2 3 4 5] <nil>
}

func ExampleFromSeq() {
	// iter.Seq[int]
	values := func(yield func(int) bool) {
		for _, v := range []int{1, 2, 3, 4, 5} {
			if !yield(v) {
				return
			}
		}
	}

	producer := rheos.FromSeq(context.TODO(), values)
	fmt.Println(rheos.Collect(producer))
	// Output: [1 2 3 4 5] <nil>
}

func ExampleAll() {
	prosucer := rheos.FromSlice(context.TODO(), []int{1, 2, 3, 4, 5})
	for i, err := range rheos.All(prosucer) {
		fmt.Println(i, err)
	}
	// Output:
	// 1 <nil>
	// 2 <nil>
	// 3 <nil>
	// 4 <nil>
	// 5 <nil>
}
