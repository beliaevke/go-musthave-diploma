package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
)

func TestUpdate(t *testing.T) {
	testCases := []struct {
		name           string
		pattern        string
		shouldPanic    bool
		method         string // Method to be used for the test request
		path           string // Path to be used for the test request
		expectedBody   string // Expected response body
		expectedStatus int    // Expected HTTP status code
	}{
		// Valid patterns
		{
			name:           "Valid pattern with HTTP POST",
			pattern:        "/update/{metricType}/{metricName}/{metricValue}",
			shouldPanic:    false,
			method:         "POST",
			path:           "/update/gauge/someMetric/11.11",
			expectedBody:   "with-prefix POST",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil && !tc.shouldPanic {
					t.Errorf("Unexpected panic for pattern %s:\n%v", tc.pattern, r)
				}
			}()

			r1 := chi.NewRouter()
			r1.Handle(tc.pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := w.Write([]byte(tc.expectedBody))
				if err != nil {
					t.Errorf("Write failed: %v", err)
				}
			}))

			// Test that HandleFunc also handles method patterns
			r2 := chi.NewRouter()
			r2.HandleFunc(tc.pattern, func(w http.ResponseWriter, r *http.Request) {
				_, err := w.Write([]byte(tc.expectedBody))
				if err != nil {
					t.Errorf("Write failed: %v", err)
				}
			})

			if !tc.shouldPanic {
				for _, r := range []chi.Router{r1, r2} {
					// Use testRequest for valid patterns
					ts := httptest.NewServer(r)
					defer ts.Close()
					resp, body := testRequest(t, ts, tc.method, tc.path, nil)
					defer resp.Body.Close()
					if body != tc.expectedBody || resp.StatusCode != tc.expectedStatus {
						t.Errorf("Expected status %d and body %s; got status %d and body %s for pattern %s",
							tc.expectedStatus, tc.expectedBody, resp.StatusCode, body, tc.pattern)
					}
				}
			}
		})
	}
}

func testRequest(t *testing.T, ts *httptest.Server, method, path string, body io.Reader) (*http.Response, string) {
	req, err := http.NewRequest(method, ts.URL+path, body)
	if err != nil {
		t.Fatal(err)
		return nil, ""
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
		return nil, ""
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
		return nil, ""
	}
	defer resp.Body.Close()

	return resp, string(respBody)
}
