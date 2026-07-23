package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	energy := 0

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Energy CLI")
	fmt.Println("Commands: add N, spend N, show, reset, double, status, exit")

	for {
		fmt.Print("energy> ")

		if !scanner.Scan() {
			fmt.Println()
			break
		}

		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		command := strings.ToLower(parts[0])

		switch command {
		case "add":
			amount, err := parseAmount(parts)
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}

			newEnergy, err := addEnergy(energy, amount)
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}

			energy = newEnergy
			fmt.Println("Energy:", energy)

		case "spend":
			amount, err := parseAmount(parts)
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}

			newEnergy, err := spendEnergy(energy, amount)
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}

			energy = newEnergy
			fmt.Println("Energy:", energy)

		case "show":
			fmt.Println("Energy:", energy)

		case "reset":
			energy = 0
			fmt.Println("Energy reset to 0")

		case "double":
			energy = energy * 2
			fmt.Println("Energy:", energy)

		case "status":
			if energy == 0 {
				fmt.Println("Energy status: empty")
			} else if energy < 10 {
				fmt.Println("Energy status: low")
			} else {
				fmt.Println("Energy status: high")
			}

		case "exit", "quit":
			fmt.Println("Goodbye")
			return

		case "help":
			fmt.Println(
				"Commands: add N, spend N, show, reset, double, status, exit",
			)

		default:
			fmt.Println("Unknown command:", command)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Input error:", err)
	}
}

func parseAmount(parts []string) (int, error) {
	if len(parts) != 2 {
		return 0, errors.New("expected command and amount, for example: add 5")
	}

	amount, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", parts[1], err)
	}

	return amount, nil
}

func addEnergy(current int, amount int) (int, error) {
	if amount <= 0 {
		return current, errors.New("amount must be greater than zero")
	}

	return current + amount, nil
}

func spendEnergy(current int, amount int) (int, error) {
	if amount <= 0 {
		return current, errors.New("amount must be greater than zero")
	}

	if amount > current {
		return current, fmt.Errorf(
			"not enough energy: have %d, need %d",
			current,
			amount,
		)
	}

	return current - amount, nil
}
