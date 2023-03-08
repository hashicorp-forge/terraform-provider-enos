package nomad

import "testing"

func TestGetTaskLogsResponse_GetLogFileName(t *testing.T) {
	t.Parallel()
	type fields struct {
		Namespace  string
		Allocation string
		Task       string
		Logs       []byte
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
				Namespace:  "taco-truck",
				Allocation: "make.taco[0]",
				Task:       "chicken",
			},
			args: args{prefix: "taco-builder"},
			want: "taco-builder_taco-truck_make.taco[0]_chicken.log",
		},
		{
			name: "with_prefix_no_namespace",
			fields: fields{
				Allocation: "make.taco[0]",
				Task:       "chicken",
			},
			args: args{prefix: "taco-builder"},
			want: "taco-builder_make.taco[0]_chicken.log",
		},
		{
			name: "without_prefix",
			fields: fields{
				Namespace:  "taco-truck",
				Allocation: "make.taco[0]",
				Task:       "chicken",
			},
			want: "taco-truck_make.taco[0]_chicken.log",
		},
		{
			name: "without_prefix_no_namespace",
			fields: fields{
				Allocation: "make.taco[0]",
				Task:       "chicken",
			},
			want: "make.taco[0]_chicken.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &GetTaskLogsResponse{
				Namespace:  tt.fields.Namespace,
				Allocation: tt.fields.Allocation,
				Task:       tt.fields.Task,
				Logs:       tt.fields.Logs,
			}
			if got := r.GetLogFileName(tt.args.prefix); got != tt.want {
				t.Errorf("GetLogFileName() = %v, want %v", got, tt.want)
			}
		})
	}
}
