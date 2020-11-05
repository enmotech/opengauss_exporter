// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"fmt"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	type args struct {
		configPath string
	}
	tests := []struct {
		name        string
		args        args
		wantQueries map[string]*QueryInstance
		wantErr     bool
	}{
		{
			name:        "default",
			args:        args{configPath: "../../og_exporter_default.yaml"},
			wantQueries: defaultMonList,
		},
		{
			name: "null",
			args: args{configPath: ""},
			// wantQueries: defaultMonList,
			wantErr: true,
		},
		{
			name:        "dir",
			args:        args{configPath: "../.."},
			wantQueries: defaultMonList,
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQueries, err := LoadConfig(tt.args.configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for _, v := range tt.wantQueries {
				_ = v.Check()

			}
			for k, v := range gotQueries {
				fmt.Println(k, v.ToYaml())
			}
			// assert.Equal(t,defaultMonList,gotQueries)
		})
	}
}

func TestParseConfig(t *testing.T) {
	type args struct {
		content []byte
		path    string
	}
	tests := []struct {
		name        string
		args        args
		wantQueries map[string]*QueryInstance
		wantErr     bool
	}{
		{
			name: "pg_bgwriter",
			args: args{
				content: []byte(`pg_bgwriter:
  desc: OpenGauss background writer metrics
  query:
  - name: pg_stat_bgwriter
    sql: |-
      SELECT checkpoints_timed,
          checkpoints_req,
          checkpoint_write_time,
          checkpoint_sync_time,
          buffers_checkpoint,
          buffers_clean,
          buffers_backend,
          maxwritten_clean,
          buffers_backend_fsync,
          buffers_alloc,
          stats_reset
      FROM pg_stat_bgwriter
    version: '>=0.0.0'
    status: enable
  metrics:
  - name: checkpoints_timed
    description: scheduled checkpoints that have been performed
    usage: COUNTER
  - name: checkpoints_req
    description: requested checkpoints that have been performed
    usage: COUNTER
  - name: checkpoint_write_time
    description: time spending on writing files to disk, in µs
    usage: COUNTER
  - name: checkpoint_sync_time
    description: time spending on syncing files to disk, in µs
    usage: COUNTER
  - name: buffers_checkpoint
    description: buffers written during checkpoints
    usage: COUNTER
  - name: buffers_clean
    description: buffers written by the background writer
    usage: COUNTER
  - name: buffers_backend
    description: buffers written directly by a backend
    usage: COUNTER
  - name: maxwritten_clean
    description: times that bgwriter stopped a cleaning scan
    usage: COUNTER
  - name: buffers_backend_fsync
    description: times a backend had to execute its own fsync
    usage: COUNTER
  - name: buffers_alloc
    description: buffers allocated
    usage: COUNTER
  - name: stats_reset
    description: time when statistics were last reset
    usage: COUNTER
  status: enable
  timeout: 0.1`),
				path: "",
			},
			wantQueries: map[string]*QueryInstance{
				"pg_bgwriter": pgStatBgWriter,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQueries, err := ParseConfig(tt.args.content, tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for _, v := range tt.wantQueries {
				_ = v.Check()

			}
			for k, v := range gotQueries {
				fmt.Println(k, v.ToYaml())
			}
			// assert.e(t,tt.wantQueries,gotQueries)
			// assert.eq
		})
	}
}
