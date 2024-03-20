// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package systemd

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSSHUnitJournalResponse_GetLogFileName(t *testing.T) {
	t.Parallel()

	s := GetUnitJournalResponse{
		Unit: "taco-truck",
		Host: "10.0.0.1",
		Logs: []byte{},
	}
	logFile := s.GetLogFileName()

	assert.Equal(t, "taco-truck_10.0.0.1.log", logFile)
}

func Test_parseServiceInfos(t *testing.T) {
	t.Parallel()

	type args struct {
		services string
	}
	tests := []struct {
		name string
		args args
		want []ServiceInfo
	}{
		{
			name: "with_systemctl_output",
			args: args{
				services: `console-setup.service                          loaded    active   exited  Set console font and keymap
consul.service                                 loaded    active   running HashiCorp Consul - A service mesh solution
cron.service                                   loaded    active   running Regular background program processing daemon`,
			},
			want: []ServiceInfo{
				{
					Unit:        "console-setup",
					Load:        "loaded",
					Active:      "active",
					Sub:         "exited",
					Description: "Set console font and keymap",
				},
				{
					Unit:        "consul",
					Load:        "loaded",
					Active:      "active",
					Sub:         "running",
					Description: "HashiCorp Consul - A service mesh solution",
				},
				{
					Unit:        "cron",
					Load:        "loaded",
					Active:      "active",
					Sub:         "running",
					Description: "Regular background program processing daemon",
				},
			},
		},
		{
			name: "empty_output",
			args: args{
				services: "",
			},
			want: []ServiceInfo{},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := parseServiceInfos(tt.args.services); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseServiceInfos() = %v, want %v", got, tt.want)
			}
		})
	}
}
