package artifactory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
)

type Client struct {
	http     http.Client
	Host     string
	Username string
	Token    string
	mu       sync.Mutex
}

type Opt func(*Client) *Client

func NewClient(opts ...Opt) *Client {
	c := &Client{
		mu: sync.Mutex{},
	}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

func WithHost(domain string) Opt {
	return func(client *Client) *Client {
		client.Host = domain
		return client
	}
}

func WithUsername(username string) Opt {
	return func(client *Client) *Client {
		client.Username = username
		return client
	}
}

func WithToken(token string) Opt {
	return func(client *Client) *Client {
		client.Token = token
		return client
	}
}

func (c *Client) SearchAQL(ctx context.Context, req *SearchAQLRequest) (*SearchAQLResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	res := &SearchAQLResponse{}

	search, err := url.Parse(fmt.Sprintf("%s/api/search/aql", c.Host))
	if err != nil {
		return res, fmt.Errorf("generating search url: %w", err)
	}

	buf := bytes.Buffer{}
	err = req.QueryTemplate.Execute(&buf, req)
	if err != nil {
		return res, fmt.Errorf("generating AQL query template: %w", err)
	}

	areq, err := http.NewRequestWithContext(ctx, "POST", search.String(), &buf)
	if err != nil {
		return res, fmt.Errorf("generating artifactory search request: %w", err)
	}
	areq.Header.Add("Content-Type", "text/plain")
	areq.SetBasicAuth(c.Username, c.Token)

	ares, err := c.http.Do(areq)
	if err != nil {
		return res, fmt.Errorf("executing artifactory search query: %w", err)
	}
	defer ares.Body.Close()

	body, err := io.ReadAll(ares.Body)
	if err != nil {
		return res, fmt.Errorf("reading artifactory search query response body: %w", err)
	}

	if ares.StatusCode != 200 {
		return res, fmt.Errorf("executing artifactory search query: %s - %s", ares.Status, string(body))
	}

	err = json.Unmarshal(body, &res)
	if err != nil {
		return res, fmt.Errorf("marshaling artifactory search query response: %w", err)
	}

	return res, nil
}
