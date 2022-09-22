package nomad

import (
	"bytes"
	"context"
	"fmt"

	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/nomad/api"
)

// Client a wrapper for the Nomad API Client
type Client struct {
	client *api.Client
}

// ClientCfg the configuration required for the Client
type ClientCfg struct {
	Host     string
	SecretID string
}

// ExecRequestOpts exec options for a Nomad exec request
type ExecRequestOpts struct {
	AllocationID string
	TaskName     string
	Command      []string
	StdIn        bool
}

// execRequest Nomad based implementation of an it.ExecRequest
type execRequest struct {
	client  *Client
	opts    ExecRequestOpts
	streams *it.ExecStreams
}

func (e *execRequest) Streams() *it.ExecStreams {
	return e.streams
}

func NewExecRequest(client *Client, opts ExecRequestOpts) it.ExecRequest {
	return &execRequest{
		client:  client,
		opts:    opts,
		streams: it.NewExecStreams(opts.StdIn),
	}
}

func (e *execRequest) Exec(ctx context.Context) *it.ExecResponse {
	return e.client.Exec(ctx, e.opts, e.streams)
}

func NewClient(cfg ClientCfg) (*Client, error) {
	client, err := createClient(cfg)
	if err != nil {
		return nil, err
	}

	return &Client{client: client}, nil
}

// GetAllocation gets the allocation for the provided prefix. The prefix can be either a complete
// allocID or a prefix portion of the ID. For example:
//
//	GetAllocation("4a212111") and
//	GetAllocation("4a212111-7d45-6ad4-0ad8-abe2d8661248")
//
// will both return the same result, so long as there are not more than one alloc with the prefix: 4a212111
func (c *Client) GetAllocation(allocPrefix string) (*api.Allocation, error) {
	allocs, _, err := c.client.Allocations().PrefixList(allocPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list allocs by prefix: %s, due to: %w", allocPrefix, err)
	}

	count := len(allocs)
	if count != 1 {
		return nil, fmt.Errorf("failed to find allocations for prefix: %s, expected 1, got: %d", allocPrefix, count)
	}

	info, _, err := c.client.Allocations().Info(allocs[0].ID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to find allocation for id: %s, due to: %w", allocPrefix, err)
	}

	return info, nil
}

// Exec performs a Nomad based remote exec
func (c *Client) Exec(ctx context.Context, opts ExecRequestOpts, streams *it.ExecStreams) *it.ExecResponse {
	response := it.NewExecResponse()

	select {
	case <-ctx.Done():
		response.ExecErr <- ctx.Err()
		return response
	default:
	}

	response.Stdout = streams.Stdout()
	response.Stderr = streams.Stderr()

	stdin := streams.Stdin()
	if stdin == nil {
		stdin = bytes.NewReader(nil)
	}

	execFn := func() {
		defer streams.Close()

		alloc, err := c.GetAllocation(opts.AllocationID)
		if err != nil {
			response.ExecErr <- err
			return
		}

		exitCode, err := c.client.Allocations().Exec(
			ctx,
			alloc,
			opts.TaskName,
			false,
			opts.Command,
			stdin,
			streams.StdoutWriter(),
			streams.StderrWriter(),
			nil,
			&api.QueryOptions{},
		)
		execErr := err
		if exitCode != 0 {
			if execErr == nil {
				// nomad does not return an error when the command exits with an exit code other than 0,
				// so we're adding one to be consistent with the other transports
				execErr = fmt.Errorf("command terminated with exit code %d", exitCode)
			}
			execErr = it.NewExecError(execErr, exitCode)
		}
		response.ExecErr <- execErr
	}

	go execFn()

	return response
}

func (c *Client) Close() {
	c.client.Close()
}

// createClient creates the Nomad API client
func createClient(opts ClientCfg) (*api.Client, error) {
	config := &api.Config{
		Address: opts.Host,
	}

	if len(opts.SecretID) > 0 {
		config.SecretID = opts.SecretID
	}

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create the nomad client due to: %w", err)
	}
	return client, nil
}
