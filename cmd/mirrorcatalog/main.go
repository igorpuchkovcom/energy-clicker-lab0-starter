package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Mirror struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
	Region  string `json:"region"`
}

func (m Mirror) Summary() string {
	status := "disabled"

	if m.Enabled {
		status = "enabled"
	}

	return fmt.Sprintf(
		"%s -> %s [%s] %s",
		m.ID,
		m.URL,
		status,
		m.Region,
	)
}

func (m *Mirror) Enable() {
	m.Enabled = true
}

func (m *Mirror) Disable() {
	m.Enabled = false
}

type Catalog struct {
	Mirrors []Mirror
	byID    map[string]*Mirror
}

func NewCatalog(mirrors []Mirror) (*Catalog, error) {
	catalog := &Catalog{
		Mirrors: mirrors,
		byID:    make(map[string]*Mirror, len(mirrors)),
	}

	for index := range catalog.Mirrors {
		mirror := &catalog.Mirrors[index]

		if mirror.ID == "" {
			return nil, fmt.Errorf(
				"mirror at index %d has no id",
				index,
			)
		}

		if mirror.URL == "" {
			return nil, fmt.Errorf(
				"mirror %q has no url",
				mirror.ID,
			)
		}

		if _, exists := catalog.byID[mirror.ID]; exists {
			return nil, fmt.Errorf(
				"duplicate mirror id %q",
				mirror.ID,
			)
		}

		catalog.byID[mirror.ID] = mirror
	}

	return catalog, nil
}

func (c *Catalog) Find(id string) (*Mirror, bool) {
	mirror, exists := c.byID[id]
	return mirror, exists
}

func (c *Catalog) CountEnabled() int {
	count := 0

	for _, mirror := range c.Mirrors {
		if mirror.Enabled {
			count++
		}
	}

	return count
}

func loadCatalog(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(
			"read catalog file: %w",
			err,
		)
	}

	var mirrors []Mirror

	if err := json.Unmarshal(data, &mirrors); err != nil {
		return nil, fmt.Errorf(
			"decode catalog JSON: %w",
			err,
		)
	}

	return NewCatalog(mirrors)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println(
			"Usage: mirrorcatalog <catalog.json>",
		)
		os.Exit(2)
	}

	catalog, err := loadCatalog(os.Args[1])
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Mirror Catalog")
	fmt.Println(
		"Commands: list, show ID, enable ID, disable ID, stats, exit, enabled, regions",
	)

	for {
		fmt.Print("catalog> ")

		if !scanner.Scan() {
			fmt.Println()
			break
		}

		parts := strings.Fields(scanner.Text())

		if len(parts) == 0 {
			continue
		}

		command := strings.ToLower(parts[0])

		switch command {
		case "list":
			printMirrors(catalog.Mirrors)

		case "show":
			id, err := parseMirrorID(parts)
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}

			mirror, exists := catalog.Find(id)
			if !exists {
				fmt.Println("Mirror not found:", id)
				continue
			}

			if err := printJSON(mirror); err != nil {
				fmt.Println("Error:", err)
			}

		case "enable":
			id, err := parseMirrorID(parts)
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}

			mirror, exists := catalog.Find(id)
			if !exists {
				fmt.Println("Mirror not found:", id)
				continue
			}

			mirror.Enable()
			fmt.Println("Enabled", mirror.ID)

		case "disable":
			id, err := parseMirrorID(parts)
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}

			mirror, exists := catalog.Find(id)
			if !exists {
				fmt.Println("Mirror not found:", id)
				continue
			}

			mirror.Disable()
			fmt.Println("Disabled", mirror.ID)

		case "stats":
			enabled := catalog.CountEnabled()
			total := len(catalog.Mirrors)
			disabled := total - enabled

			fmt.Println("Total:", total)
			fmt.Println("Enabled:", enabled)
			fmt.Println("Disabled:", disabled)

		case "enabled":
			for _, mirror := range catalog.Mirrors {
				if mirror.Enabled {
					fmt.Println(mirror.Summary())
				}
			}

		case "regions":
			counts := make(map[string]int)

			for _, mirror := range catalog.Mirrors {
				counts[mirror.Region]++
			}

			for region, count := range counts {
				fmt.Printf("%s: %d\n", region, count)
			}

		case "exit", "quit":
			fmt.Println("Goodbye")
			return

		default:
			fmt.Println("Unknown command:", command)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Input error:", err)
	}
}

func printMirrors(mirrors []Mirror) {
	if len(mirrors) == 0 {
		fmt.Println("Catalog is empty")
		return
	}

	for index, mirror := range mirrors {
		fmt.Printf(
			"%d. %s\n",
			index+1,
			mirror.Summary(),
		)
	}
}

func parseMirrorID(parts []string) (string, error) {
	if len(parts) != 2 {
		return "", errors.New(
			"expected mirror id, for example: show mirror-a",
		)
	}

	return parts[1], nil
}

func printJSON(value any) error {
	data, err := json.MarshalIndent(
		value,
		"",
		"  ",
	)
	if err != nil {
		return fmt.Errorf(
			"encode JSON: %w",
			err,
		)
	}

	fmt.Println(string(data))
	return nil
}
