// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/file"

	"github.com/hashicorp/go-multierror"

	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/command"
)

// TransportTestSuite a test suite that can be used to test any transport.
type TransportTestSuite struct {
	suite.Suite
	transportFn func(t *testing.T) it.Transport
}

func NewTransportTestSuite(transportFn func(t *testing.T) it.Transport) *TransportTestSuite {
	s := new(TransportTestSuite)
	s.transportFn = transportFn

	return s
}

func (s *TransportTestSuite) TestRun() {
	t := s.T()
	transport := s.transportFn(t)
	t.Parallel()

	type args struct {
		command it.Command
	}
	tests := []struct {
		name         string
		args         args
		wantStdout   string
		wantStderr   string
		wantErrs     []string
		wantExitCode int
	}{
		{
			name: "no_error",
			args: args{
				command: command.New("for KEY in `seq 1 10`; do echo $KEY; done"),
			},
			wantStderr: "",
			wantStdout: "1\n2\n3\n4\n5\n6\n7\n8\n9\n10",
		},
		{
			name: "error_exit_1",
			args: args{
				command: command.New("echo \"exit 1\" > /tmp/run_exit_1; chmod +x /tmp/run_exit_1; /tmp/run_exit_1"),
			},
			wantStderr:   "",
			wantStdout:   "",
			wantErrs:     []string{"command terminated with exit code 1"},
			wantExitCode: 1,
		},
		{
			name: "error_stderr",
			args: args{
				command: command.New(">&2 echo \"something failed sucka\""),
			},
			wantStderr: "something failed sucka",
			wantStdout: "",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t1 *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			gotStdout, gotStderr, err := transport.Run(ctx, tt.args.command)
			cancel()
			if tt.wantExitCode != 0 {
				var exitErr *it.ExecError
				require.ErrorAs(t, err, &exitErr)
				require.Equal(t1, tt.wantExitCode, exitErr.ExitCode())
				err = exitErr.Unwrap()
			}
			var merr *multierror.Error
			if errors.As(err, &merr) {
				gotErrors := merr.WrappedErrors()
				if len(gotErrors) != len(tt.wantErrs) {
					t1.Errorf("Copy() errors = %s, wantErrs %s", gotErrors, tt.wantErrs)
				} else {
					for i, err := range gotErrors {
						if err.Error() != tt.wantErrs[i] {
							t1.Errorf("Copy() error = %v, wantErr %v", err.Error(), tt.wantErrs[i])
						}
					}
				}
			}
			if gotStdout != tt.wantStdout {
				t1.Errorf("Run() gotStdout = %v, want %v", gotStdout, tt.wantStdout)
			}
			if gotStderr != tt.wantStderr {
				t1.Errorf("Run() gotStderr = %v, want %v", gotStderr, tt.wantStderr)
			}
		})
	}
}

func (s *TransportTestSuite) TestCopy() {
	t := s.T()
	transport := s.transportFn(t)
	t.Parallel()

	type args struct {
		ctx      context.Context
		copyable it.Copyable
		dst      string
	}
	tests := []struct {
		name         string
		args         args
		wantErrs     []*regexp.Regexp
		wantExitCode int
	}{
		{
			name: "no_error",
			args: args{
				ctx:      context.TODO(),
				copyable: file.NewReader("This is some content\x00"),
				dst:      "/tmp/file.txt",
			},
		},
		{
			name: "read_error",
			args: args{
				ctx:      context.TODO(),
				copyable: file.NewErrorCopyable("failed to read data"),
				dst:      "/tmp/file.txt",
			},
			wantErrs: []*regexp.Regexp{
				regexp.MustCompile(`^command terminated with exit code 1$`),
				regexp.MustCompile(`^failed to read data$`),
				regexp.MustCompile(`^failed to copy to dst: \[/tmp/file.txt], due to: \[.*]$`),
			},
			wantExitCode: 1,
		},
		{
			name: "bad_destination",
			args: args{
				ctx:      context.TODO(),
				copyable: file.NewReader("This is some content\x00"),
				dst:      "/etc",
			},
			wantErrs: []*regexp.Regexp{
				regexp.MustCompile(`^command terminated with exit code 1$`),
				regexp.MustCompile(`failed to copy to dst: \[/etc], due to: \[tar: can't remove old file etc:.*]$`),
			},
			wantExitCode: 1,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t1 *testing.T) {
			err := transport.Copy(tt.args.ctx, tt.args.copyable, tt.args.dst)
			if tt.wantExitCode != 0 {
				var exitErr *it.ExecError
				require.ErrorAs(t1, err, &exitErr)
				require.Equal(t1, tt.wantExitCode, exitErr.ExitCode())
				err = exitErr.Unwrap()
			}
			if merr, ok := err.(*multierror.Error); ok {
				gotErrors := merr.WrappedErrors()
				if len(gotErrors) != len(tt.wantErrs) {
					t1.Errorf("Copy() errors = %s, wantErrs %s", gotErrors, tt.wantErrs)
				} else {
					for i, err := range gotErrors {
						if !tt.wantErrs[i].MatchString(err.Error()) {
							t1.Errorf("Copy() error = %v, wantErr %v", err.Error(), tt.wantErrs[i])
						}
					}
				}
			}
		})
	}
}

func (s *TransportTestSuite) TestStream() {
	t := s.T()
	transport := s.transportFn(t)
	t.Parallel()

	type args struct {
		ctx     context.Context
		command it.Command
	}
	tests := []struct {
		name         string
		args         args
		wantStdout   string
		wantStderr   string
		wantErr      string
		wantExitCode int
	}{
		{
			name: "no_error",
			args: args{
				ctx:     context.TODO(),
				command: command.New("for KEY in `seq 1 10`; do echo $KEY; done"),
			},
			wantStderr: "",
			wantStdout: "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n",
		},
		{
			name: "error_exit_1",
			args: args{
				ctx:     context.TODO(),
				command: command.New("echo \"exit 1\" > /tmp/run_exit_1; chmod +x /tmp/run_exit_1; /tmp/run_exit_1"),
			},
			wantStderr:   "",
			wantStdout:   "",
			wantErr:      "command terminated with exit code 1",
			wantExitCode: 1,
		},
		{
			name: "error_stderr",
			args: args{
				ctx:     context.TODO(),
				command: command.New(">&2 echo \"something failed sucka\""),
			},
			wantStderr: "something failed sucka\n",
			wantStdout: "",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t1 *testing.T) {
			stdOut, stdErr, errC := transport.Stream(tt.args.ctx, tt.args.command)

			wg := sync.WaitGroup{}
			wg.Add(2)
			readErrs := make(chan error, 2)
			var gotStdOut string
			go func() {
				outBytes, err := io.ReadAll(stdOut)
				if err != nil {
					readErrs <- err
				}
				gotStdOut = string(outBytes)
				wg.Done()
			}()

			var gotStdErr string
			go func() {
				errBytes, err := io.ReadAll(stdErr)
				if err != nil {
					readErrs <- err
				}
				gotStdErr = string(errBytes)
				wg.Done()
			}()

			err := <-errC

			wg.Wait()

			close(readErrs)
			for readErr := range readErrs {
				require.NoError(t, readErr)
			}

			if !reflect.DeepEqual(gotStdOut, tt.wantStdout) {
				t1.Errorf("Stream() gotStdout = %v, want %v", gotStdOut, tt.wantStdout)
			}
			if !reflect.DeepEqual(gotStdErr, tt.wantStderr) {
				t1.Errorf("Stream() gotStderr = %v, want %v", gotStdErr, tt.wantStderr)
			}

			if tt.wantExitCode != 0 {
				var exitErr *it.ExecError
				require.ErrorAs(t1, err, &exitErr)
				require.Equal(t1, tt.wantExitCode, exitErr.ExitCode())
				err = exitErr.Unwrap()
			}
			if (err != nil && err.Error() != tt.wantErr) || (err != nil && tt.wantErr == "") {
				t1.Errorf("Stream() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
