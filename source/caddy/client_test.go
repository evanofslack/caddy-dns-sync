package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/evanofslack/caddy-dns-sync/source"
)

// MockHttpClient implements the Httper interface for testing
type MockHttpClient struct {
	GetFunc func(url string) (*http.Response, error)
}

func (m *MockHttpClient) Do(req *http.Request) (*http.Response, error) {
	return m.GetFunc(req.URL.Host)
}

func TestDomains(t *testing.T) {
	adminURL := "http://localhost:2019"

	tests := []struct {
		name           string
		mockResponse   interface{}
		mockStatusCode int
		mockError      error
		expected       []source.DomainConfig
		expectError    bool
	}{
		{
			name: "successful domains extraction",
			mockResponse: map[string]interface{}{
				"apps": map[string]interface{}{
					"http": map[string]interface{}{
						"servers": map[string]interface{}{
							"main": map[string]interface{}{
								"listen": []string{":80", ":443"},
								"routes": []map[string]interface{}{
									{
										"match": []map[string]interface{}{
											{
												"host": []string{"example.com", "www.example.com"},
											},
										},
										"handle": []map[string]interface{}{
											{
												"handler": "reverse_proxy",
												"upstreams": []map[string]interface{}{
													{
														"dial": "localhost:8080",
													},
												},
											},
										},
									},
									{
										"match": []map[string]interface{}{
											{
												"host": []string{"api.example.com"},
											},
										},
										"handle": []map[string]interface{}{
											{
												"handler": "reverse_proxy",
												"upstreams": []map[string]interface{}{
													{
														"dial": "localhost:9000",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			mockStatusCode: http.StatusOK,
			mockError:      nil,
			expected: []source.DomainConfig{
				{Host: "example.com", Upstream: "localhost:8080"},
				{Host: "www.example.com", Upstream: "localhost:8080"},
				{Host: "api.example.com", Upstream: "localhost:9000"},
			},
			expectError: false,
		},
		{
			name:           "http request error",
			mockResponse:   nil,
			mockStatusCode: 0,
			mockError:      errors.New("connection refused"),
			expected:       []source.DomainConfig{},
			expectError:    true,
		},
		{
			name:           "non-200 status code",
			mockResponse:   nil,
			mockStatusCode: http.StatusInternalServerError,
			mockError:      nil,
			expected:       []source.DomainConfig{},
			expectError:    true,
		},
		{
			name:           "invalid json response",
			mockResponse:   "invalid json",
			mockStatusCode: http.StatusOK,
			mockError:      nil,
			expected:       []source.DomainConfig{},
			expectError:    true,
		},
		{
			name: "empty configuration",
			mockResponse: map[string]interface{}{
				"apps": map[string]interface{}{
					"http": map[string]interface{}{
						"servers": map[string]interface{}{},
					},
				},
			},
			mockStatusCode: http.StatusOK,
			mockError:      nil,
			expected:       []source.DomainConfig{},
			expectError:    false,
		},
		{
			name: "nested configuration parsing",
			mockResponse: map[string]interface{}{
				"apps": map[string]interface{}{
					"http": map[string]interface{}{
						"servers": map[string]interface{}{
							"srv0": map[string]interface{}{
								"listen": []string{":443"},
								"routes": []map[string]interface{}{{
									"match": []map[string]interface{}{{"host": []string{"*.eslack.net"}}},
									"handle": []map[string]interface{}{{
										"handler": "subroute",
										"routes": []map[string]interface{}{{
											"match": []map[string]interface{}{{"host": []string{"synctest.local.eslack.net"}}},
											"handle": []map[string]interface{}{{
												"handler": "subroute",
												"routes": []map[string]interface{}{{
													"handle": []map[string]interface{}{{
														"handler":   "reverse_proxy",
														"upstreams": []map[string]interface{}{{"dial": "1.1.1.1:443"}},
													}},
												}},
											}},
										}},
									}},
									"terminal": true,
								}},
							},
						},
					},
				},
			},
			mockStatusCode: http.StatusOK,
			expected: []source.DomainConfig{
				{Host: "synctest.local.eslack.net", Upstream: "1.1.1.1:443"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			// Create a mock HTTP client
			mockClient := &MockHttpClient{
				GetFunc: func(url string) (*http.Response, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}

					var respBody []byte
					var err error

					if tt.mockResponse != nil {
						if s, ok := tt.mockResponse.(string); ok {
							respBody = []byte(s)
						} else {
							respBody, err = json.Marshal(tt.mockResponse)
							if err != nil {
								t.Fatalf("Failed to marshal mock response: %v", err)
							}
						}
					}

					return &http.Response{
						StatusCode: tt.mockStatusCode,
						Body:       io.NopCloser(bytes.NewReader(respBody)),
					}, nil
				},
			}

			// Create client with mock HTTP client
			c := &client{
				adminURL: adminURL,
				http:     mockClient,
			}

			// Call the method being tested
			result, err := c.Domains(ctx)

			// Check for expected error
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Compare results
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected domains %+v but got %+v", tt.expected, result)
			}
		})
	}
}
