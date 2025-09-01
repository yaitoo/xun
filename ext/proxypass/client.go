package proxypass

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/yaitoo/xun/ext/sse"
)

type Client struct {
	code       string
	connectURL string
	apiUser    string
	apiPasswd  string
	client     *http.Client
}

func NewClient(code, connectURL, apiUser, apiPasswd string) *Client {
	return &Client{
		code:       code,
		connectURL: connectURL,
		apiUser:    apiUser,
		apiPasswd:  apiPasswd,
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				Proxy: http.ProxyFromEnvironment,
			},
		},
	}
}

func (c *Client) Connect(ctx context.Context, domain string, targetURL string) error {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.connectURL, http.NoBody)
	if err != nil {
		return err
	}

	req.SetBasicAuth(c.apiUser, c.apiPasswd)
	req.Header.Set("X-Code", c.code)
	req.Header.Set("X-Domain", domain)
	req.Header.Set("X-Target", targetURL)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	reader := sse.NewReader(resp.Body)

	for {
		_, err := reader.Next()
		if err != nil {
			return err
		}
	}
}
