package v13

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	httpClient "github.com/infinispan/infinispan-operator/pkg/http"
)

const LoggersPath = BasePath + "/logging/loggers"

type logging struct {
	httpClient.HttpClient
}

func (l *logging) GetLoggers() (lm map[string]string, err error) {
	rsp, err := l.Get(LoggersPath, nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()

	if err = httpClient.ValidateResponse(rsp, err, "getting cluster loggers", http.StatusOK); err != nil {
		return
	}

	type Logger struct {
		Name  string `json:"name"`
		Level string `json:"level"`
	}
	var loggers []Logger
	if err := json.NewDecoder(rsp.Body).Decode(&loggers); err != nil {
		return nil, fmt.Errorf("unable to decode: %w", err)
	}
	lm = make(map[string]string)
	for _, logger := range loggers {
		if logger.Name != "" {
			lm[logger.Name] = logger.Level
		}
	}
	return
}

func (l *logging) SetLogger(name, level string) error {
	path := fmt.Sprintf("%s/%s?level=%s", LoggersPath, name, strings.ToUpper(level))
	rsp, err := l.Put(path, "", nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()

	if err := httpClient.ValidateResponse(rsp, err, "setting cluster logger", http.StatusNoContent); err != nil {
		return err
	}
	return nil
}
