// Package http provides a basic http interface that can be used by higher-level abstractions that need to execute http
// requests but do not care about the underlying host or configuration.
package http

import (
	"fmt"
	"io"
	"net/http"

	"github.com/infinispan/infinispan-operator/controllers/constants"
)

// HttpClient interface containing methods for the most common HTTP methods
type HttpClient interface {
	Head(path string, headers map[string]string) (*http.Response, error)
	Get(path string, headers map[string]string) (*http.Response, error)
	Post(path, payload string, headers map[string]string) (*http.Response, error)
	PostMultipart(path string, parts map[string]string, headers map[string]string) (*http.Response, error)
	Put(path, payload string, headers map[string]string) (*http.Response, error)
	Delete(path string, headers map[string]string) (*http.Response, error)
}

// HttpError Error() implementation
type HttpError struct {
	Status  int
	Message string
}

func (e *HttpError) Error() string {
	return fmt.Sprintf("unexpected HTTP status code (%d): %s", e.Status, e.Message)
}

// ValidateResponse utility function to ensure that a returned http response has a valid status code
func ValidateResponse(rsp *http.Response, inperr error, entity string, validCodes ...int) (err error) {
	if inperr != nil {
		return fmt.Errorf("unexpected error %s: %w", entity, inperr)
	}

	if rsp == nil || len(validCodes) == 0 {
		return
	}

	for _, code := range validCodes {
		if code == rsp.StatusCode {
			return
		}
	}

	defer func() {
		cerr := rsp.Body.Close()
		if err == nil {
			err = cerr
		}
	}()

	responseBody, responseErr := io.ReadAll(rsp.Body)
	if responseErr != nil {
		return &HttpError{
			Status:  rsp.StatusCode,
			Message: fmt.Sprintf("server side error %s. Unable to read response body: %s", entity, responseErr.Error()),
		}
	}
	return &HttpError{
		Status:  rsp.StatusCode,
		Message: fmt.Sprintf("unexpected error %s, response: %s", entity, constants.GetWithDefault(string(responseBody), rsp.Status)),
	}
}

// CloseBody utility function to ensure that a http.Response body is correctly closed
func CloseBody(rsp *http.Response, err error) error {
	var cerr error
	if rsp != nil {
		cerr = rsp.Body.Close()
	}
	if err == nil {
		return cerr
	}
	return err
}
