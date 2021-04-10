// Copyright © 2021 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_genDSNString(t *testing.T) {
	type args struct {
		connStringSettings map[string]string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "a1",
			args: args{
				connStringSettings: map[string]string{
					"host":     "localhost",
					"password": "passwordDsn",
					"port":     "55432",
					"sslmode":  "disabled",
					"user":     "userDsn",
				},
			},
			want: "host=localhost password=passwordDsn port=55432 sslmode=disabled user=userDsn",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := genDSNString(tt.args.connStringSettings); got != tt.want {
				t.Errorf("genDSNString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isIPOnly(t *testing.T) {
	type args struct {
		host string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "localhost:55432",
			args: args{"localhost:55432"},
			want: false,
		},
		{
			name: "localhost",
			args: args{"localhost"},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isIPOnly(tt.args.host); got != tt.want {
				t.Errorf("isIPOnly() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseDSNSettings(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{
		{
			name: "localhost:55432",
			args: args{
				s: "postgres://userDsn:passwordDsn@localhost:55432/?sslmode=disabled",
			},
			want: map[string]string{
				"postgres://userDsn:passwordDsn@localhost:55432/?sslmode": "disabled",
			},
			wantErr: false,
		},
		{
			name: "err",
			args: args{
				s: "user",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "127.0.0.1:5432",
			args: args{
				s: "user=xxx password=xxx host=127.0.0.1 port=5432 dbname=postgres sslmode=disable",
			},
			want: map[string]string{
				"database": "postgres",
				"host":     "127.0.0.1",
				"password": "xxx",
				"port":     "5432",
				"sslmode":  "disable",
				"user":     "xxx",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDSNSettings(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDSNSettings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_parseDsn(t *testing.T) {
	type args struct {
		dsn string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{
		{
			name: "localhost:55432",
			args: args{
				dsn: "postgres://userDsn:passwordDsn@localhost:55432/?sslmode=disabled",
			},
			want: map[string]string{
				"host":     "localhost",
				"password": "passwordDsn",
				"port":     "55432",
				"sslmode":  "disabled",
				"user":     "userDsn",
			},
		},
		{
			name: "localhost:55432",
			args: args{
				dsn: "postgres://userDsn:passwordDsn%3D@localhost:55432/?sslmode=disabled",
			},
			want: map[string]string{
				"host":     "localhost",
				"password": "passwordDsn=",
				"port":     "55432",
				"sslmode":  "disabled",
				"user":     "userDsn",
			},
		},
		{
			name: "127.0.0.1:5432",
			args: args{
				dsn: "user=xxx password=xxx host=127.0.0.1 port=5432 dbname=postgres sslmode=disable",
			},
			want: map[string]string{
				"database": "postgres",
				"host":     "127.0.0.1",
				"password": "xxx",
				"port":     "5432",
				"sslmode":  "disable",
				"user":     "xxx",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDsn(tt.args.dsn)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDsn() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_parseURLSettings(t *testing.T) {
	type args struct {
		connString string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{
		{
			name: "localhost:55432",
			args: args{
				connString: "postgres://userDsn:passwordDsn@localhost:55432/?sslmode=disabled",
			},
			want: map[string]string{
				"host":     "localhost",
				"password": "passwordDsn",
				"port":     "55432",
				"sslmode":  "disabled",
				"user":     "userDsn",
			},
			wantErr: false,
		},
		{
			name: "127.0.0.1:5432",
			args: args{
				connString: "user=xxx password=xxx host=127.0.0.1 port=5432 dbname=postgres sslmode=disable",
			},
			want: map[string]string{
				"database": "user=xxx password=xxx host=127.0.0.1 port=5432 dbname=postgres sslmode=disable",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseURLSettings(tt.args.connString)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseURLSettings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
