[![Go Reference](https://pkg.go.dev/badge/github.com/dmksnnk/rheos.svg)](https://pkg.go.dev/github.com/dmksnnk/rheos)
![GitHub tag (with filter)](https://img.shields.io/github/v/tag/dmksnnk/rheos)
![GitHub Workflow Status (with event)](https://img.shields.io/github/actions/workflow/status/dmksnnk/rheos/go.yml)

# rheos

rheos (from Greek "rheos," meaning a stream or current) is like [lo](https://github.com/samber/lo), but async. 
It provides building blocks for asynchronous stream processing:

- Cancellation of stream processing on context cancellation or error.
- Functional goodies: mapping, filtering, reducing, batching and collecting of stream elements.

## Installation

Only Go 1.18+ is supported.

```bash
go get -u gitlab.heu.group/dmknnk/rheos
````

## Usage

A simple example of showing how to map a stream of integers to strings and squish them together:

```go
producer := rheos.FromSlice(context.Background(), []int{1, 2, 3, 4, 5})
strings := rheos.Map(producer, func(_ context.Context, v int) (string, error) {
    return strconv.Itoa(v), nil
})
got, err := rheos.Reduce(
    strings,
    func(acc string, s string) (string, error) {
        return acc + s, nil
    },
    "",
)
fmt.Println(got, err)
// Output: 12345 <nil> 
```

Fetching URL asynchronously:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
producer := rheos.FromSlice(ctx, []string{
    "https://example.com/1",
    "https://example.com/2",
    "https://example.com/3",
})
responses := rheos.Map(producer, func(ctx context.Context, url string) (*http.Response, error) {
    return http.Get(url)
})
bodies := rheos.Map(responses, func(ctx context.Context, resp *http.Response) ([]byte, error) {
    defer resp.Body.Close()
    return io.ReadAll(resp.Body)
})
err := rheos.ForEach(bodies, func(ctx context.Context, body []byte) error {
    fmt.Println(body)
    return nil
})
if err != nil {
    log.Fatal(err)
}
````
Pipeline cancellation when context is cancelled:

```go
ctx, cancel := context.WithCancel(context.Background())
producer := rheos.FromSlice(ctx, []int{1, 2, 3, 4, 5})
strings := rheos.Map(producer, func(_ context.Context, v int) (string, error) {
    cancel()
    return strconv.Itoa(v), nil
})
got, err := rheos.Collect(strings)
fmt.Println(got, err)
// Output: [] context canceled
```

Pipeline cancellation when error is encountered:

```go
producer := rheos.FromSlice(context.Background(), []int{1, 2, 3, 4, 5})
strings := rheos.Map(producer, func(_ context.Context, v int) (string, error) {
    return "", errors.New("I'm an error")
})
got, err := rheos.Collect(strings)
fmt.Println(got, err)
// Output: [] I'm an error
```

For more examples see [examples](example_test.go).
