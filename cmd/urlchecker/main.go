package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
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
	URL        string
	WorkerID   int
	StatusCode int
	Status     string
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
	if len(args) < 1 || len(args) > 2 {
		return errors.New(
			"usage: urlchecker <sequential|parallel|pool> [workers]",
		)
	}

	mode := args[0]
	workers := 2

	if len(args) == 2 {
		parsed, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf(
				"parse workers %q: %w",
				args[1],
				err,
			)
		}

		if parsed < 1 {
			return errors.New(
				"workers must be greater than zero",
			)
		}

		workers = parsed
	}

	jobs := defaultJobs()

	client := &http.Client{
		Timeout: 700 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	started := time.Now()

	var (
		results []Result
		err     error
	)

	switch mode {
	case "sequential":
		results = checkSequential(
			ctx,
			client,
			jobs,
		)

	case "parallel":
		results = checkParallel(
			ctx,
			client,
			jobs,
		)

	case "pool":
		results, err = checkWorkerPool(
			ctx,
			client,
			jobs,
			workers,
		)
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf(
			"unknown mode %q",
			mode,
		)
	}

	printResults(results)

	fmt.Println(
		"total:",
		time.Since(started).Round(time.Millisecond),
	)

	return nil
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
			Name:  "hanging",
			URL:   "http://localhost:18084/healthz",
		},
		{
			Index: 4,
			Name:  "slow",
			URL:   "http://localhost:18085/healthz",
		},
	}
}

func checkURL(
	ctx context.Context,
	client *http.Client,
	job Job,
) Result {
	result := Result{
		Index: job.Index,
		Name:  job.Name,
		URL:   job.URL,
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

	_, _ = io.Copy(
		io.Discard,
		io.LimitReader(response.Body, 4096),
	)

	result.StatusCode = response.StatusCode
	result.Status = response.Status

	return result
}

func checkSequential(
	ctx context.Context,
	client *http.Client,
	jobs []Job,
) []Result {
	results := make(
		[]Result,
		0,
		len(jobs),
	)

	for _, job := range jobs {
		result := checkURL(
			ctx,
			client,
			job,
		)

		results = append(
			results,
			result,
		)
	}

	return results
}

func checkParallel(
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

			resultCh <- checkURL(
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

func checkWorkerPool(
	ctx context.Context,
	client *http.Client,
	jobs []Job,
	workers int,
) ([]Result, error) {
	jobCh := make(chan Job)
	resultCh := make(chan Result)

	var wg sync.WaitGroup

	for workerID := 1; workerID <= workers; workerID++ {
		wg.Add(1)

		go worker(
			ctx,
			workerID,
			client,
			jobCh,
			resultCh,
			&wg,
		)
	}

	go func() {
		defer close(jobCh)

		for _, job := range jobs {
			select {
			case jobCh <- job:

			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	results := make(
		[]Result,
		len(jobs),
	)

	received := 0

	for {
		select {
		case result, ok := <-resultCh:
			if !ok {
				if received != len(jobs) {
					return results, fmt.Errorf(
						"received %d of %d results",
						received,
						len(jobs),
					)
				}

				return results, nil
			}

			results[result.Index] = result
			received++

		case <-ctx.Done():
			return results, ctx.Err()
		}
	}
}

func worker(
	ctx context.Context,
	workerID int,
	client *http.Client,
	jobs <-chan Job,
	results chan<- Result,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return

		case job, ok := <-jobs:
			if !ok {
				return
			}

			result := checkURL(
				ctx,
				client,
				job,
			)

			result.WorkerID = workerID

			select {
			case results <- result:

			case <-ctx.Done():
				return
			}
		}
	}
}

func printResults(results []Result) {
	for _, result := range results {
		workerText := ""

		if result.WorkerID > 0 {
			workerText = fmt.Sprintf(
				" worker=%d",
				result.WorkerID,
			)
		}

		if result.Err != nil {
			fmt.Printf(
				"%-10s%s ERROR after %s: %v\n",
				result.Name,
				workerText,
				result.Latency.Round(
					time.Millisecond,
				),
				result.Err,
			)

			continue
		}

		fmt.Printf(
			"%-10s%s status=%d latency=%s\n",
			result.Name,
			workerText,
			result.StatusCode,
			result.Latency.Round(
				time.Millisecond,
			),
		)
	}
}
