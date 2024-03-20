// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package kubernetes

import "testing"

func TestGetPodLogsResponse_GetLogFileName(t *testing.T) {
	t.Parallel()

	type fields struct {
		ContextName string
		Namespace   string
		Pod         string
		Container   string
		Logs        []byte
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "all_fields",
			fields: fields{
				ContextName: "taco-truck",
				Namespace:   "food",
				Pod:         "mexican",
				Container:   "tacos",
			},
			want: "taco-truck_food_mexican_tacos.log",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := GetPodLogsResponse{
				ContextName: tt.fields.ContextName,
				Namespace:   tt.fields.Namespace,
				Pod:         tt.fields.Pod,
				Container:   tt.fields.Container,
			}
			if got := p.GetLogFileName(); got != tt.want {
				t.Errorf("GetLogFileName() = %v, want %v", got, tt.want)
			}
		})
	}
}
