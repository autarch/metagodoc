package indexer

import (
	"net/http"
)

type GHTransport struct {
	token string
	*http.Transport
}

func NewGHTransport(token string) *GHTransport {
	return &GHTransport{token, &http.Transport{}}
}

func (t *GHTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	req.SetBasicAuth(t.token, "x-oauth-basic")
	return t.Transport.RoundTrip(req)
}
