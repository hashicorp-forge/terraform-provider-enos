package kubernetes

import (
	"context"

	it "github.com/hashicorp/enos-provider/internal/transport"
)

// MockClient a mock Kubernetes Client
type MockClient struct {
	NewExecRequestFunc func(opts ExecRequestOpts) it.ExecRequest
	QueryPodInfosFunc  func(ctx context.Context, req QueryPodInfosRequest) ([]PodInfo, error)
	GetPodInfoFunc     func(ctx context.Context, req GetPodInfoRequest) (*PodInfo, error)
	GetLogsFunc        func(ctx context.Context, req GetPodLogsRequest) (*GetPodLogsResponse, error)
}

func (m *MockClient) NewExecRequest(opts ExecRequestOpts) it.ExecRequest {
	return m.NewExecRequestFunc(opts)
}

func (m *MockClient) QueryPodInfos(ctx context.Context, req QueryPodInfosRequest) ([]PodInfo, error) {
	return m.QueryPodInfosFunc(ctx, req)
}

func (m *MockClient) GetPodInfo(ctx context.Context, req GetPodInfoRequest) (*PodInfo, error) {
	return m.GetPodInfoFunc(ctx, req)
}

func (m *MockClient) GetLogs(ctx context.Context, req GetPodLogsRequest) (*GetPodLogsResponse, error) {
	return m.GetLogsFunc(ctx, req)
}

// NewMockGetLogsFunc creates a GetLogsFunc that returns the provided logs when called.
func NewMockGetLogsFunc(logs []byte) func(ctx context.Context, req GetPodLogsRequest) (*GetPodLogsResponse, error) {
	return func(ctx context.Context, req GetPodLogsRequest) (*GetPodLogsResponse, error) {
		return &GetPodLogsResponse{
			ContextName: req.ContextName,
			Namespace:   req.Namespace,
			Pod:         req.Pod,
			Container:   req.Container,
			Logs:        logs,
		}, nil
	}
}
