// package http

// astra/utils/http/httputils.go (new)
package httputils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func PostJSON(url string, body interface{}, resp interface{}) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}
	r, err := http.Post(url, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %d", r.StatusCode)
	}
	if resp != nil {
		return json.NewDecoder(r.Body).Decode(resp)
	}
	return nil
}

func PostStream(url string, body interface{}) (io.ReadCloser, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	r, err := http.Post(url, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	if r.StatusCode != http.StatusOK {
		defer r.Body.Close()
		return nil, fmt.Errorf("bad status: %d", r.StatusCode)
	}
	return r.Body, nil
}
