package v13

import (
	"bytes"
	"fmt"
	"net/http"

	httpClient "github.com/infinispan/infinispan-operator/pkg/http"
	"github.com/infinispan/infinispan-operator/pkg/mime"
)

const MetricsPath = "metrics"

type metrics struct {
	httpClient.HttpClient
}

func (m *metrics) Get(postfix string) (buf *bytes.Buffer, err error) {
	headers := map[string]string{
		"Acccept": string(mime.ApplicationJson),
	}

	path := fmt.Sprintf("%s/%s", MetricsPath, postfix)
	rsp, err := m.HttpClient.Get(path, headers)
	if err = httpClient.ValidateResponse(rsp, err, "getting metrics", http.StatusOK); err != nil {
		return
	}

	defer func() {
		cerr := rsp.Body.Close()
		if err == nil {
			err = cerr
		}
	}()

	buf = new(bytes.Buffer)
	if _, err = buf.ReadFrom(rsp.Body); err != nil {
		return
	}
	return
}
