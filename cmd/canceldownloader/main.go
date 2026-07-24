package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Job struct {
	Index int
	Name  string
	URL   string
}

type Result struct {
	Index      int
	Name       string
	StatusCode int
	Status     string
	Bytes      int
	Latency    time.Duration
	Err        error
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return errors.New(
			"usage: cancellable-downloader <timeout>",
		)
	}

	timeout, err := time.ParseDuration(args[0])
	if err != nil {
		return fmt.Errorf(
			"parse timeout %q: %w",
			args[0],
			err,
		)
	}

	if timeout <= 0 {
		return errors.New(
			"timeout must be greater than zero",
		)
	}

	// Корневой context без timeout и отмены.
	background := context.Background()

	// Этот context отменяется вручную при Ctrl+C.
	cancelContext, cancelAll := context.WithCancel(
		background,
	)
	defer cancelAll()

	installSignalHandler(
		cancelContext,
		cancelAll,
	)

	// Этот дочерний context отменяется по timeout
	// либо раньше, если отменился cancelContext.
	ctx, cancelTimeout := context.WithTimeout(
		cancelContext,
		timeout,
	)
	defer cancelTimeout()

	go observeCancellation(ctx)

	client := &http.Client{}

	jobs := defaultJobs()

	fmt.Println("Starting downloads")
	fmt.Println("Group timeout:", timeout)
	fmt.Println("Press Ctrl+C to cancel")
	fmt.Println()

	started := time.Now()

	results := downloadAll(
		ctx,
		client,
		jobs,
	)

	printResults(results)

	fmt.Println()
	fmt.Println(
		"total:",
		time.Since(started).Round(time.Millisecond),
	)

	fmt.Println(
		"context result:",
		contextResult(ctx),
	)

	return nil
}

func installSignalHandler(
	ctx context.Context,
	cancel context.CancelFunc,
) {
	signalCh := make(
		chan os.Signal,
		1,
	)

	signal.Notify(
		signalCh,
		os.Interrupt,
		syscall.SIGTERM,
	)

	go func() {
		defer signal.Stop(signalCh)

		select {
		case receivedSignal := <-signalCh:
			fmt.Fprintln(
				os.Stderr,
				"\nReceived signal:",
				receivedSignal,
			)

			cancel()

		case <-ctx.Done():
			return
		}
	}()
}

func observeCancellation(ctx context.Context) {
	<-ctx.Done()

	fmt.Fprintln(
		os.Stderr,
		"Context finished:",
		ctx.Err(),
	)
}

func defaultJobs() []Job {
	return []Job{
		{
			Index: 0,
			Name:  "good",
			URL:   "http://localhost:18081/healthz",
		},
		{
			Index: 1,
			Name:  "broken",
			URL:   "http://localhost:18082/healthz",
		},
		{
			Index: 2,
			Name:  "not-ready",
			URL:   "http://localhost:18083/readyz",
		},
		{
			Index: 3,
			Name:  "slow",
			URL:   "http://localhost:18085/healthz",
		},
		{
			Index: 4,
			Name:  "hanging",
			URL:   "http://localhost:18084/healthz",
		},
	}
}

func downloadAll(
	ctx context.Context,
	client *http.Client,
	jobs []Job,
) []Result {
	resultCh := make(
		chan Result,
		len(jobs),
	)

	var wg sync.WaitGroup

	for _, job := range jobs {
		wg.Add(1)

		go func(job Job) {
			defer wg.Done()

			resultCh <- download(
				ctx,
				client,
				job,
			)
		}(job)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	results := make(
		[]Result,
		len(jobs),
	)

	for result := range resultCh {
		results[result.Index] = result
	}

	return results
}

func download(
	ctx context.Context,
	client *http.Client,
	job Job,
) Result {
	result := Result{
		Index: job.Index,
		Name:  job.Name,
	}

	// Не начинаем новую операцию, если context
	// уже был отменён.
	select {
	case <-ctx.Done():
		result.Err = ctx.Err()
		return result

	default:
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		job.URL,
		nil,
	)
	if err != nil {
		result.Err = fmt.Errorf(
			"create request: %w",
			err,
		)
		return result
	}

	started := time.Now()

	response, err := client.Do(request)

	result.Latency = time.Since(started)

	if err != nil {
		result.Err = err
		return result
	}

	defer response.Body.Close()

	body, err := io.ReadAll(
		io.LimitReader(
			response.Body,
			64*1024,
		),
	)
	if err != nil {
		result.Err = fmt.Errorf(
			"read response body: %w",
			err,
		)
		return result
	}

	result.StatusCode = response.StatusCode
	result.Status = response.Status
	result.Bytes = len(body)

	return result
}

func printResults(results []Result) {
	for _, result := range results {
		if result.Err != nil {
			fmt.Printf(
				"%-10s %-10s after %s: %v\n",
				result.Name,
				errorKind(result.Err),
				result.Latency.Round(
					time.Millisecond,
				),
				result.Err,
			)

			continue
		}

		fmt.Printf(
			"%-10s status=%d bytes=%d latency=%s\n",
			result.Name,
			result.StatusCode,
			result.Bytes,
			result.Latency.Round(
				time.Millisecond,
			),
		)
	}
}

func errorKind(err error) string {
	switch {
	case errors.Is(
		err,
		context.DeadlineExceeded,
	):
		return "TIMEOUT"

	case errors.Is(
		err,
		context.Canceled,
	):
		return "CANCELLED"

	default:
		return "ERROR"
	}
}

func contextResult(ctx context.Context) string {
	switch ctx.Err() {
	case nil:
		return "still active"

	case context.Canceled:
		return "cancelled"

	case context.DeadlineExceeded:
		return "deadline exceeded"

	default:
		return ctx.Err().Error()
	}
}
