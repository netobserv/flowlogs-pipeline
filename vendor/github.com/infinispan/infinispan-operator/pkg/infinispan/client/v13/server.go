package v13

import (
	"net/http"

	httpClient "github.com/infinispan/infinispan-operator/pkg/http"
)

const ServerPath = BasePath + "/server"

type server struct {
	httpClient.HttpClient
}

func (s *server) Stop() (err error) {
	rsp, err := s.Post(ServerPath+"?action=stop", "", nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	err = httpClient.ValidateResponse(rsp, err, "stopping server", http.StatusNoContent)
	return
}
