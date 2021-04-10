// Copyright © 2021 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"testing"
)

func TestErrorConnectToServer_Error(t *testing.T) {
	type fields struct {
		Msg string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "a1",
			fields: fields{Msg: "a1"},
			want:   "a1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &ErrorConnectToServer{
				Msg: tt.fields.Msg,
			}
			if got := e.Error(); got != tt.want {
				t.Errorf("Error() = %v, want %v", got, tt.want)
			}
		})
	}
}
