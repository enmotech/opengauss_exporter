// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"testing"
)

func Test_RateLimit(t *testing.T) {
	type args struct {
		n int
	}
	tests := []struct {
		name string
		args args
		want *rateLimit
	}{
		{
			name: "RateLimit",
			args: args{n: 30},
			want: &rateLimit{token: make(chan struct{}, 30)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newRateLimit(tt.args.n)
			got.getToken()
			got.putToken()

		})
	}
}
