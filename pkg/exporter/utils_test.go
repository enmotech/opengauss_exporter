// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"github.com/blang/semver"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func Test_parseConstLabels(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want prometheus.Labels
	}{
		{
			name: "a=b",
			args: args{s: "a=b"},
			want: prometheus.Labels{
				"a": "b",
			},
		},
		{
			name: "null",
			args: args{s: ""},
			want: nil,
		},
		{
			name: "a=b, c=d",
			args: args{s: "a=b, c=d"},
			want: prometheus.Labels{
				"a": "b",
				"c": "d",
			},
		},
		{
			name: "a=b, xyz",
			args: args{s: "a=b, xyz"},
			want: prometheus.Labels{
				"a": "b",
			},
		},
		{
			name: "a=",
			args: args{s: "a="},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseConstLabels(tt.args.s)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShadowDSN(t *testing.T) {
	type args struct {
		dsn string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "localhost:55432",
			args: args{
				dsn: "postgres://userDsn:passwordDsn@localhost:55432/?sslmode=disabled",
			},
			want: "postgres://userDsn:%2A%2A%2A%2A%2A%2A@localhost:55432/?sslmode=disabled",
		},
		{
			name: "localhost:55432",
			args: args{
				dsn: "postgres://gaussdb:Test@123@127.0.0.1:5432/postgres?sslmode=disable",
			},
			want: "postgres://gaussdb:%2A%2A%2A%2A%2A%2A@127.0.0.1:5432/postgres?sslmode=disable",
		},
		{
			name: "localhost:55432",
			args: args{
				dsn: "postgres://userDsn:xxxxx@localhost:55432/?sslmode=disabled",
			},
			want: "postgres://userDsn:%2A%2A%2A%2A%2A%2A@localhost:55432/?sslmode=disabled",
		},
		{
			name: "127.0.0.1:5432",
			args: args{
				dsn: "user=xxx password=xxx host=127.0.0.1 port=5432 dbname=postgres sslmode=disable",
			},
			want: "user=xxx%20password=xxx%20host=127.0.0.1%20port=5432%20dbname=postgres%20sslmode=disable",
		},
		{
			name: "localhost:1234",
			args: args{
				dsn: "port=1234",
			},

			want: "port=1234",
		},
		{
			name: "example:5432",
			args: args{
				dsn: "host=example",
			},
			want: "host=example",
		},
		{
			name: "xyz",
			args: args{
				dsn: "xyz",
			},
			want: "xyz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShadowDSN(tt.args.dsn); got != tt.want {
				t.Errorf("ShadowDSN() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	type args struct {
		a []string
		x string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Contains",
			args: args{
				a: []string{"a", "b"},
				x: "a",
			},
			want: true,
		},
		{
			name: "Not Contains",
			args: args{
				a: []string{"a", "b"},
				x: "c",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Contains(tt.args.a, tt.args.x); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseCSV(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name     string
		args     args
		wantTags []string
	}{
		{
			name:     "parseCSV",
			args:     args{s: "a1=a1,b1=b1"},
			wantTags: []string{"a1=a1", "b1=b1"},
		},
		{
			name:     "nil",
			args:     args{s: ""},
			wantTags: nil,
		},
		{
			name:     ",",
			args:     args{s: ","},
			wantTags: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotTags := parseCSV(tt.args.s); !reflect.DeepEqual(gotTags, tt.wantTags) {
				t.Errorf("parseCSV() = %v, want %v", gotTags, tt.wantTags)
			}
		})
	}
}

func Test_parseVersionSem(t *testing.T) {
	type args struct {
		versionString string
	}
	tests := []struct {
		name    string
		args    args
		want    semver.Version
		wantErr bool
	}{
		{
			name: "(openGauss 1.0.0 build",
			args: args{versionString: "(openGauss 1.0.0 build"},
			want: semver.Version{
				Major: 1,
				Minor: 0,
				Patch: 0,
				Pre:   nil,
				Build: nil,
			},
		},
		{
			name:    "aaaaa",
			args:    args{versionString: "aaaa"},
			want:    semver.Version{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVersionSem(tt.args.versionString)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseVersionSem() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseVersionSem() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseVersion(t *testing.T) {
	type args struct {
		versionString string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// {
		// 	name: "EnterpriseDB 9.6.5.10",
		// 	args: args{versionString: "EnterpriseDB 9.6.5.10 on x86_64-pc-linux-gnu, compiled by gcc (GCC) 4.4.7 20120313 (Red Hat 4.4.7-16), 64-bit"},
		// 	want: "9.6.5",
		// },
		// {
		// 	name: "postgres 9.5.4",
		// 	args: args{versionString: "postgres 9.5.4, compiled by Visual C++ build 1800, 64-bit"},
		// 	want: "9.5.4",
		// },
		{
			name: "1.0.0",
			args: args{versionString: "(openGauss 1.0.0 build 5ed8dc17) compiled at 2020-09-15 18:04:09 commit 0 last mr   on x86_64-unknown-linux-gnu, compiled by g++ (GCC) 8.2.0, 64-"},
			want: "1.0.0",
		},
		{
			name: "1.0.1",
			args: args{versionString: "(openGauss 1.0.1 build 89d339ca) compiled at 2020-12-21 11:12:55 commit 0 last mr   on aarch64-unknown-linux-gnu, compiled by g++ (GCC) 8.2.0, 64-bit"},
			want: "1.0.1",
		},
		{
			name: "1.1.0",
			args: args{versionString: "PostgreSQL 9.2.4 (openGauss 1.1.0 build 392c0438) compiled at 2020-12-31 20:07:42 commit 0 last mr   on x86_64-unknown-linux-gnu, compiled by g++ (GCC) 7.3.0, 64-bit"},
			want: "1.1.0",
		},
		{
			name: "MogDB_1.1.0",
			args: args{versionString: "PostgreSQL 9.2.4 (MogDB 1.1.0 build fffb972f) compiled at 2021-03-08 15:01:26 commit 0 last mr   on aarch64-unknown-linux-gnu, compiled by g++ (GCC) 7.3.0, 64-bit"},
			want: "1.1.0",
		},
		{
			name: "og_2.0.0",
			args: args{versionString: "PostgreSQL 9.2.4 (openGauss 2.0.0 build 78689da9) compiled at 2021-03-31 21:04:03 commit 0 last mr   on x86_64-unknown-linux-gnu, compiled by g++ (GCC) 7.3.0, 64-bit"},
			want: "2.0.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseVersion(tt.args.versionString); got != tt.want {
				t.Errorf("parseVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}
