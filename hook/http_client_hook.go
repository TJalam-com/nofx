package hook

import (
	"log"
	"net/http"
)

type SetHttpClientResult struct {
	Err    error
	Client *http.Client
}

func (r *SetHttpClientResult) Error() error {
	if r.Err != nil {
		log.Printf("⚠️ Error executing NewAsterTraderResult: %v", r.Err)
	}
	return r.Err
}

func (r *SetHttpClientResult) GetResult() *http.Client {
	r.Error()
	return r.Client
}
