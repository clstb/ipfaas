package openfaas

import "net/http"

type Auth func(req *http.Request)

type Client struct {
	Host   string
	Client *http.Client
	Auth   Auth
}

type Opt func(*Client)

func NewClient(host string, opts ...Opt) *Client {
	c := &Client{
		Host:   host,
		Client: &http.Client{},
		Auth:   func(req *http.Request) {},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func WithBasicAuth(username, password string) func(c *Client) {
	return func(c *Client) {
		c.Auth = func(req *http.Request) {
			req.SetBasicAuth(username, password)
		}
	}
}
