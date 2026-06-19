package resolver

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/miekg/dns"
)

type Resolver struct {
	url    string
	client *http.Client
}

func NewResolver(url string, client *http.Client) *Resolver {
	return &Resolver{
		url:    url,
		client: client,
	}
}

func (r *Resolver) Resolve(dnsmessage []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", r.url, bytes.NewReader(dnsmessage))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/dns-message")
	req.Header.Set("Content-Type", "application/dns-message")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream DoH returned status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	msg := new(dns.Msg)
	if err := msg.Unpack(body); err != nil {
		return nil, fmt.Errorf("invalid upstream DNS response: %w", err)
	}

	return body, err
}
