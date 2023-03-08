package systemd

import (
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
