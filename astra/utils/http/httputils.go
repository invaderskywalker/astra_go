// astra/utils/http/httputils.go
package httputils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// PostJSON sends a standard POST request with a JSON body
// and decodes the response into `resp` if provided.
func PostJSON(url string, body interface{}, resp interface{}) error {
	return PostJSONContext(context.Background(), url, body, resp)
}

// PostJSONContext is the cancellable form used by agent requests. The context
// must reach the HTTP request so stopping an agent also stops local inference.
func PostJSONContext(ctx context.Context, url string, body interface{}, resp interface{}) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r.Body)
		return fmt.Errorf("bad status: %d - %s", r.StatusCode, string(b))
	}

	if resp != nil {
		return json.NewDecoder(r.Body).Decode(resp)
	}
	return nil
}

// PostStream sends a POST request and returns the raw response body for streaming.
// Caller is responsible for closing the returned io.ReadCloser.
func PostStream(url string, body interface{}) (io.ReadCloser, error) {
	return PostStreamContext(context.Background(), url, body)
}

// PostStreamContext returns a response body whose underlying request is
// cancelled when ctx is cancelled.
func PostStreamContext(ctx context.Context, url string, body interface{}) (io.ReadCloser, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if r.StatusCode != http.StatusOK {
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		return nil, fmt.Errorf("bad status: %d - %s", r.StatusCode, string(b))
	}

	return r.Body, nil
}

// PostJSONWithAuth sends a JSON POST request with Bearer authentication.
// It decodes the response into respDest if provided.
func PostJSONWithAuth(url, apiKey string, payload, respDest interface{}) error {
	reqBody := mustMarshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bad status: %d - %s", resp.StatusCode, string(b))
	}

	if respDest != nil {
		return json.NewDecoder(resp.Body).Decode(respDest)
	}
	return nil
}

// PostStreamWithAuth sends a JSON POST request with Bearer authentication
// and returns a streaming response body. Caller must close it.
func PostStreamWithAuth(url, apiKey string, payload interface{}) (io.ReadCloser, error) {
	reqBody := mustMarshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("bad status: %d - %s", resp.StatusCode, string(b))
	}

	return resp.Body, nil
}

// mustMarshal converts any Go struct/map into JSON bytes or panics on error.
// Safe helper for internal use.
func mustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Errorf("failed to marshal JSON: %w", err))
	}
	return b
}
