package kubernetes

import "testing"

func TestGetPodLogsResponse_GetLogFileName(t *testing.T) {
	t.Parallel()

	type fields struct {
		Namespace string
		Pod       string
		Container string
		Logs      []byte
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
			name: "all_fields_no_prefix",
			fields: fields{
				Namespace: "food",
				Pod:       "mexican",
				Container: "tacos",
			},
			want: "food_mexican_tacos.log",
		},
		{
			name: "all_fields_with_prefix",
			fields: fields{
				Namespace: "food",
				Pod:       "mexican",
				Container: "tacos",
			},
			args: args{
				prefix: "some-prefix",
			},
			want: "some-prefix_food_mexican_tacos.log",
		},
		{
			name: "no_namespace_no_prefix",
			fields: fields{
				Pod:       "mexican",
				Container: "tacos",
			},
			want: "mexican_tacos.log",
		},
		{
			name: "no_namespace_with_prefix",
			fields: fields{
				Pod:       "mexican",
				Container: "tacos",
			},
			args: args{
				prefix: "some-prefix",
			},
			want: "some-prefix_mexican_tacos.log",
		},
		{
			name: "no_namespace_no_container_no_prefix",
			fields: fields{
				Pod: "mexican",
			},
			want: "mexican.log",
		},
		{
			name: "no_namespace_no_container_with_prefix",
			fields: fields{
				Pod: "mexican",
			},
			args: args{
				prefix: "some-prefix",
			},
			want: "some-prefix_mexican.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := GetPodLogsResponse{
				Namespace: tt.fields.Namespace,
				Pod:       tt.fields.Pod,
				Container: tt.fields.Container,
			}
			if got := p.GetLogFileName(tt.args.prefix); got != tt.want {
				t.Errorf("GetLogFileName() = %v, want %v", got, tt.want)
			}
		})
	}
}
