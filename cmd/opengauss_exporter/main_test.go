// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestArgs_RetrieveTargetURL(t *testing.T) {
	var (
		url1 = "host=192.168.122.91 user=postgres_exporter password=postgres_exporter123 port=9832 dbname=opengauss sslmode=disable"
		url2 = "host=192.168.122.91 user=postgres_exporter password=postgres_exporter123 port=9832 dbname=opengauss sslmode=disable," +
			"host=192.168.122.91 user=postgres_exporter password=postgres_exporter123 port=9832 dbname=opengauss sslmode=disable"
	)
	type fields struct {
		DbURL     string
		EnvName   string
		EnvValues string
	}
	tests := []struct {
		name   string
		fields fields
		want   []string
	}{
		{
			name: "url1",
			fields: fields{
				DbURL: url1,
			},
			want: []string{url1},
		},
		{
			name: "url2",
			fields: fields{
				DbURL: url2,
			},
			want: strings.Split(url2, ","),
		},
		{
			name: "PG_EXPORTER_URL",
			fields: fields{
				EnvName:   "PG_EXPORTER_URL",
				EnvValues: url2,
			},
			want: strings.Split(url2, ","),
		},
		{
			name: "DATA_SOURCE_NAME",
			fields: fields{
				EnvName:   "DATA_SOURCE_NAME",
				EnvValues: url2,
			},
			want: strings.Split(url2, ","),
		},
		{
			name:   "default",
			fields: fields{},
			want:   strings.Split(defaultPGURL, ","),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Args{
				DbURL: &tt.fields.DbURL,
			}
			if tt.fields.EnvName != "" && tt.fields.EnvValues != "" {
				os.Setenv(tt.fields.EnvName, tt.fields.EnvValues)
			}

			if got := a.RetrieveTargetURL(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RetrieveTargetURL() = %v, want %v", got, tt.want)
			}
			if tt.fields.EnvName != "" && tt.fields.EnvValues != "" {
				os.Unsetenv(tt.fields.EnvName)
			}
		})
	}
}
