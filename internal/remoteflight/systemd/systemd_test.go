package systemd

import (
	"reflect"
	"testing"
)

func TestGetSSHLogsResponse_GetLogFileName(t *testing.T) {
	type fields struct {
		Host string
		Logs []byte
	}
	type args struct {
		prefix string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "with_prefix",
			fields: fields{
				Host: "10.15.5.4",
			},
			args: args{
				prefix: "vault",
			},
			want: "vault_10.15.5.4.log",
		},
		{
			name: "without_prefix",
			fields: fields{
				Host: "101.16.5.5",
			},
			want: "101.16.5.5.log",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := GetLogsResponse{
				Host: tt.fields.Host,
				Logs: tt.fields.Logs,
			}
			if got := s.GetLogFileName(tt.args.prefix); got != tt.want {
				t.Errorf("GetLogsResponse.GetLogFileName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseServiceInfos(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			if got := parseServiceInfos(tt.args.services); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseServiceInfos() = %v, want %v", got, tt.want)
			}
		})
	}
}
