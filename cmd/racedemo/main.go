package main

import (
	"fmt"
	"os"
	"sync"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("usage: racedemo <unsafe|safe>")
		os.Exit(2)
	}

	mode := os.Args[1]

	values := make(
		[]int,
		0,
		1000,
	)

	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < 1000; i++ {
		wg.Add(1)

		go func(value int) {
			defer wg.Done()

			switch mode {
			case "unsafe":
				values = append(
					values,
					value,
				)

			case "safe":
				mu.Lock()

				values = append(
					values,
					value,
				)

				mu.Unlock()

			default:
				return
			}
		}(i)
	}

	wg.Wait()

	fmt.Println("mode:", mode)
	fmt.Println("length:", len(values))
}
