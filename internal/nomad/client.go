package nomad

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/nomad/api"
)

type GetTaskLogsRequest struct {
	AllocationID string
	Task         string
}

type GetTaskLogsResponse struct {
	Namespace  string
	Allocation string
	Task       string
	Logs       []byte
}

// GetLogFileName gets the name that should be used for the log file, using the following pattern:
//
//	[prefix]_[namespace]_[allocation-name]_[task-name].log, where prefix and namespace are only
//	added if they are not empty.
func (r *GetTaskLogsResponse) GetLogFileName(prefix string) string {
	var parts []string

	if prefix != "" {
		parts = append(parts, prefix)
	}

	if r.Namespace != "" {
		parts = append(parts, r.Namespace)
	}

	parts = append(parts, r.Allocation, r.Task)

	result := strings.Join(parts, "_")
	result = fmt.Sprintf("%s.log", result)

	return result
}

func (r *GetTaskLogsResponse) GetLogs() []byte {
	return r.Logs
}

// Client a wrapper around the Nomad API client, providing useful functions that the provider can use
type Client interface {
	// GetAllocation gets an allocation that matches the provided prefix
	GetAllocation(allocPrefix string) (*api.Allocation, error)
	// GetAllocationInfo Gets an Allocation given the provided ID.
	GetAllocationInfo(allocID string) (*api.Allocation, error)
	// NewExecRequest creates an exec request that can be executed later
	NewExecRequest(opts ExecRequestOpts) it.ExecRequest
	// Exec executes a remote exec now
	Exec(ctx context.Context, opts ExecRequestOpts, streams *it.ExecStreams) *it.ExecResponse
	// GetLogs gets the logs for the allocation and task as specified in the request
	GetLogs(ctx context.Context, req GetTaskLogsRequest) (*GetTaskLogsResponse, error)
	// Close closes the Client.
	Close()
}

// client a wrapper for the Nomad API Client
type client struct {
	apiClient *api.Client
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
	client  *client
	opts    ExecRequestOpts
	streams *it.ExecStreams
}

func (e *execRequest) Streams() *it.ExecStreams {
	return e.streams
}

func (c *client) NewExecRequest(opts ExecRequestOpts) it.ExecRequest {
	return &execRequest{
		client:  c,
		opts:    opts,
		streams: it.NewExecStreams(opts.StdIn),
	}
}

func (e *execRequest) Exec(ctx context.Context) *it.ExecResponse {
	return e.client.Exec(ctx, e.opts, e.streams)
}

func NewClient(cfg ClientCfg) (Client, error) {
	apiClient, err := createClient(cfg)
	if err != nil {
		return nil, err
	}

	return &client{apiClient: apiClient}, nil
}

// GetAllocation gets the allocation for the provided prefix. The prefix can be either a complete
// allocID or a prefix portion of the ID. For example:
//
//	GetAllocation("4a212111") and
//	GetAllocation("4a212111-7d45-6ad4-0ad8-abe2d8661248")
//
// will both return the same result, so long as there are not more than one alloc with the prefix: 4a212111
func (c *client) GetAllocation(allocPrefix string) (*api.Allocation, error) {
	allocs, _, err := c.apiClient.Allocations().PrefixList(allocPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list allocs by prefix: %s, due to: %w", allocPrefix, err)
	}

	count := len(allocs)
	if count != 1 {
		return nil, fmt.Errorf("failed to find allocations for prefix: %s, expected 1, got: %d", allocPrefix, count)
	}

	return c.GetAllocationInfo(allocs[0].ID)
}

// GetAllocationInfo Gets the allocation info for the provided allocation id. The provided ID must be
// the full ID not just a prefix. Use GetAllocation instead if you only have the allocation prefix.
func (c *client) GetAllocationInfo(allocID string) (*api.Allocation, error) {
	info, _, err := c.apiClient.Allocations().Info(allocID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to find allocation info for id: %s, due to: %w", allocID, err)
	}

	return info, nil
}

// Exec performs a Nomad based remote exec
func (c *client) Exec(ctx context.Context, opts ExecRequestOpts, streams *it.ExecStreams) *it.ExecResponse {
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

		exitCode, err := c.apiClient.Allocations().Exec(
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

func (c *client) GetLogs(ctx context.Context, req GetTaskLogsRequest) (*GetTaskLogsResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	alloc, err := c.GetAllocation(req.AllocationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs, due to: %w", err)
	}

	frames, errors := c.apiClient.
		AllocFS().
		Logs(alloc, false, req.Task, "stdout", "start", 0, ctx.Done(), nil)

	result := &bytes.Buffer{}

ReadFrames:
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("failed to get logs, due to: %w", ctx.Err())
		default:
			// if the context is not done, just carry on
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("failed to get logs, due to: %w", ctx.Err())
		case f := <-frames:
			if f == nil {
				break ReadFrames
			}
			result.Write(f.Data)
		case err := <-errors:
			return nil, fmt.Errorf("failed to read logs, due to: %w", err)
		}
	}
	return &GetTaskLogsResponse{
		Namespace:  alloc.Namespace,
		Allocation: alloc.Name,
		Task:       req.Task,
		Logs:       result.Bytes(),
	}, nil
}

func (c *client) Close() {
	c.apiClient.Close()
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
