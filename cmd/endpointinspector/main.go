package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const maxBodyPreview = 512

type Inspection struct {
	Scheme      string
	Hostname    string
	Port        string
	Path        string
	Status      string
	StatusCode  int
	Successful  bool
	Latency     time.Duration
	ContentType string
	BodyPreview string
	Server      string
	StatusClass string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	endpoint, timeout, err := parseArguments(args)
	if err != nil {
		return err
	}

	client := &http.Client{
		Timeout: timeout,
	}

	result, err := inspectEndpoint(client, endpoint)
	if err != nil {
		return err
	}

	printInspection(result, timeout)
	return nil
}

func parseArguments(
	args []string,
) (*url.URL, time.Duration, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, 0, errors.New(
			"usage: endpoint-inspector <URL> [timeout]",
		)
	}

	endpoint, err := parseEndpoint(args[0])
	if err != nil {
		return nil, 0, err
	}

	timeout := 2 * time.Second

	if len(args) == 2 {
		parsedTimeout, err := time.ParseDuration(args[1])
		if err != nil {
			return nil, 0, fmt.Errorf(
				"parse timeout %q: %w",
				args[1],
				err,
			)
		}

		if parsedTimeout <= 0 {
			return nil, 0, errors.New(
				"timeout must be greater than zero",
			)
		}

		timeout = parsedTimeout
	}

	return endpoint, timeout, nil
}

func parseEndpoint(rawURL string) (*url.URL, error) {
	if !strings.Contains(rawURL, "://") {
		return nil, errors.New(
			"URL must include a scheme, for example: http://localhost:18081",
		)
	}

	endpoint, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf(
			"parse URL %q: %w",
			rawURL,
			err,
		)
	}

	switch endpoint.Scheme {
	case "http", "https":
		// Поддерживаемые протоколы.

	default:
		return nil, fmt.Errorf(
			"unsupported URL scheme %q",
			endpoint.Scheme,
		)
	}

	if endpoint.Hostname() == "" {
		return nil, errors.New(
			"URL hostname is required",
		)
	}

	return endpoint, nil
}

func inspectEndpoint(
	client *http.Client,
	endpoint *url.URL,
) (Inspection, error) {
	request, err := http.NewRequest(
		http.MethodGet,
		endpoint.String(),
		nil,
	)
	if err != nil {
		return Inspection{}, fmt.Errorf(
			"create HTTP request: %w",
			err,
		)
	}

	request.Header.Set(
		"User-Agent",
		"MirrorQuest-EndpointInspector/1.0",
	)

	started := time.Now()

	response, err := client.Do(request)

	latency := time.Since(started)

	if err != nil {
		return Inspection{}, fmt.Errorf(
			"send GET request to %q after %s: %w",
			endpoint.String(),
			latency.Round(time.Microsecond),
			err,
		)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(
		io.LimitReader(
			response.Body,
			maxBodyPreview,
		),
	)
	if err != nil {
		return Inspection{}, fmt.Errorf(
			"read response body from %q: %w",
			endpoint.String(),
			err,
		)
	}

	path := endpoint.EscapedPath()
	if path == "" {
		path = "/"
	}

	return Inspection{
		Scheme:      endpoint.Scheme,
		Hostname:    endpoint.Hostname(),
		Port:        endpointPort(endpoint),
		Path:        path,
		Status:      response.Status,
		StatusCode:  response.StatusCode,
		Successful:  response.StatusCode >= 200 && response.StatusCode < 300,
		Latency:     latency,
		ContentType: response.Header.Get("Content-Type"),
		BodyPreview: strings.TrimSpace(string(body)),
		Server:      response.Header.Get("Server"),
		StatusClass: statusClass(response.StatusCode),
	}, nil
}

func endpointPort(endpoint *url.URL) string {
	if port := endpoint.Port(); port != "" {
		return port
	}

	switch endpoint.Scheme {
	case "http":
		return "80"

	case "https":
		return "443"

	default:
		return ""
	}
}

func statusClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "success"

	case code >= 300 && code < 400:
		return "redirect"

	case code >= 400 && code < 500:
		return "client error"

	case code >= 500 && code < 600:
		return "server error"

	default:
		return "unknown"
	}
}

func printInspection(
	result Inspection,
	timeout time.Duration,
) {
	fmt.Println("scheme:", result.Scheme)
	fmt.Println("hostname:", result.Hostname)
	fmt.Println("port:", result.Port)
	fmt.Println("path:", result.Path)
	fmt.Println("status:", result.Status)
	fmt.Println("successful:", result.Successful)
	fmt.Println(
		"latency:",
		result.Latency.Round(time.Microsecond),
	)
	fmt.Println("timeout:", timeout)

	if result.ContentType != "" {
		fmt.Println(
			"content-type:",
			result.ContentType,
		)
	}

	if result.BodyPreview != "" {
		fmt.Println("body:", result.BodyPreview)
	}
	if result.Server != "" {
		fmt.Println("server:", result.Server)
	}
	fmt.Println("status class:", result.StatusClass)
}
