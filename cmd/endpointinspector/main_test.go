package main

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type roundTripFunc func(
	request *http.Request,
) (*http.Response, error)

func (function roundTripFunc) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	return function(request)
}

func TestInspectEndpointWithFakeTransport(
	t *testing.T,
) {
	endpoint, err := url.Parse(
		"http://mirror.test:8080/healthz",
	)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}

	client := &http.Client{
		Transport: roundTripFunc(
			func(
				request *http.Request,
			) (*http.Response, error) {
				if request.Method != http.MethodGet {
					t.Errorf(
						"method = %s, want GET",
						request.Method,
					)
				}

				if request.URL.Path != "/healthz" {
					t.Errorf(
						"path = %s, want /healthz",
						request.URL.Path,
					)
				}

				headers := make(http.Header)

				headers.Set(
					"Content-Type",
					"application/json",
				)

				headers.Set(
					"Server",
					"fake-mirror",
				)

				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     headers,
					Body: io.NopCloser(
						strings.NewReader(
							`{"status":"alive"}`,
						),
					),
					Request: request,
				}, nil
			},
		),
	}

	result, err := inspectEndpoint(
		client,
		endpoint,
	)
	if err != nil {
		t.Fatalf(
			"inspectEndpoint() error = %v",
			err,
		)
	}

	if result.StatusCode != http.StatusOK {
		t.Errorf(
			"status code = %d, want %d",
			result.StatusCode,
			http.StatusOK,
		)
	}

	if !result.Successful {
		t.Error("Successful = false, want true")
	}

	if result.BodyPreview != `{"status":"alive"}` {
		t.Errorf(
			"body = %q",
			result.BodyPreview,
		)
	}

	if result.Hostname != "mirror.test" {
		t.Errorf(
			"hostname = %q, want mirror.test",
			result.Hostname,
		)
	}

	if result.Port != "8080" {
		t.Errorf(
			"port = %q, want 8080",
			result.Port,
		)
	}
}

func TestInspectEndpointTransportError(
	t *testing.T,
) {
	endpoint, err := url.Parse(
		"http://mirror.test/healthz",
	)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}

	networkError := errors.New(
		"fake network unavailable",
	)

	client := &http.Client{
		Transport: roundTripFunc(
			func(
				request *http.Request,
			) (*http.Response, error) {
				return nil, networkError
			},
		),
	}

	_, err = inspectEndpoint(
		client,
		endpoint,
	)

	if !errors.Is(err, networkError) {
		t.Fatalf(
			"error = %v, want wrapped network error",
			err,
		)
	}
}
