// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"fmt"
	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestCheckStatus(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "enable",
			args: args{s: statusEnable},
			want: statusEnable,
		},
		{
			name: "disable",
			args: args{s: statusDisable},
			want: statusDisable,
		},
		{
			name:    "other",
			args:    args{s: "other"},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckStatus(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("CheckStatus() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQueryInstance(t *testing.T) {
	query1 := &Query{
		Name:    "",
		SQL:     "select col1,col1,col2 from dual",
		Version: "",
		Status:  "",
	}
	queryInstance := &QueryInstance{
		Name: "test",
		Desc: "test",
		Queries: []*Query{
			query1,
		},
		Metrics: []*Column{
			{
				Name:  "col1",
				Desc:  "col1",
				Usage: LABEL,
			},
			{
				Name:  "col2",
				Desc:  "col2",
				Usage: DISCARD,
			},
			{
				Name:  "col3",
				Desc:  "col3",
				Usage: GAUGE,
			},
			{
				Name:  "col4",
				Desc:  "col4",
				Usage: COUNTER,
			},
			{
				Name:  "col5",
				Desc:  "col5",
				Usage: HISTOGRAM,
			},
			{
				Name:  "col6",
				Desc:  "col6",
				Usage: MappedMETRIC,
			},
			{
				Name:  "col7",
				Desc:  "col7",
				Usage: DURATION,
			},
		},
		Status:      "",
		TTL:         0,
		Priority:    0,
		Timeout:     0,
		Path:        "",
		Columns:     nil,
		ColumnNames: nil,
		LabelNames:  nil,
		MetricNames: nil,
	}
	t.Run("Check_Timeout<0", func(t *testing.T) {
		queryInstance.Timeout = -1
		err := queryInstance.Check()
		assert.NoError(t, err)
		queryInstance.Timeout = 0.1
	})
	t.Run("Check_Status_err", func(t *testing.T) {
		queryInstance.Status = "other"
		err := queryInstance.Check()
		assert.Error(t, err)
		queryInstance.Status = statusEnable
	})
	t.Run("Check_Query_Status_err", func(t *testing.T) {
		queryInstance.Queries[0].Status = "other"
		err := queryInstance.Check()
		assert.Error(t, err)
		queryInstance.Queries[0].Status = ""
	})
	t.Run("Check_Metric_Usage_err", func(t *testing.T) {
		queryInstance.Metrics[0].Usage = "other"
		err := queryInstance.Check()
		assert.Error(t, err)
		queryInstance.Metrics[0].Usage = LABEL
	})
	t.Run("Check", func(t *testing.T) {
		err := queryInstance.Check()
		assert.NoError(t, err)
	})

	t.Run("TimeoutDuration", func(t *testing.T) {
		r := queryInstance.TimeoutDuration()
		assert.Equal(t, time.Duration(float64(time.Second)*queryInstance.Timeout), r)
	})
	t.Run("IsEnableCache", func(t *testing.T) {
		r := queryInstance.IsEnableCache()
		assert.Equal(t, false, r)
	})
	t.Run("TimeoutDuration_other", func(t *testing.T) {
		queryInstance.Timeout = 1
		r := queryInstance.TimeoutDuration()
		assert.Equal(t, time.Duration(float64(time.Second)*queryInstance.Timeout), r)
	})
	t.Run("GetQuerySQL", func(t *testing.T) {
		ver1 := semver.Version{
			Major: 0,
			Minor: 0,
			Patch: 0,
		}
		q := queryInstance.GetQuerySQL(ver1, false)
		assert.NotNil(t, q)
	})
	t.Run("GetQuerySQL_versionRange_is_null", func(t *testing.T) {
		queryInstance := &QueryInstance{
			Queries: []*Query{
				{
					SQL:     "select primary",
					DbRole:  "primary",
					Version: ">=0.0.0 < 2.0.0",
				},
				{
					SQL:     "select standby",
					DbRole:  "standby",
					Version: ">=0.0.0 < 2.0.0",
				},
				{
					SQL:     "select primary 2.0.0",
					Version: ">=2.0.0",
					DbRole:  "primary",
				},
				{
					SQL:    "select standby 2.0.0",
					DbRole: "standby",
				},
			},
		}
		ver1 := semver.Version{
			Major: 1,
			Minor: 1,
			Patch: 0,
		}
		queryInstance.Check()
		q := queryInstance.GetQuerySQL(ver1, true)
		assert.NotNil(t, q)
		assert.Equal(t, "select primary", q.SQL)
		q = queryInstance.GetQuerySQL(ver1, false)
		assert.NotNil(t, q)
		assert.Equal(t, "select standby", q.SQL)
		ver1 = semver.Version{
			Major: 2,
			Minor: 0,
			Patch: 0,
		}
		q = queryInstance.GetQuerySQL(ver1, true)
		assert.NotNil(t, q)
		assert.Equal(t, "select primary 2.0.0", q.SQL)
		q = queryInstance.GetQuerySQL(ver1, false)
		assert.NotNil(t, q)
		assert.Equal(t, "select standby 2.0.0", q.SQL)

	})
	t.Run("GetColumn", func(t *testing.T) {
		c := queryInstance.GetColumn("col1", nil)
		assert.NotNil(t, c)
		col2 := queryInstance.GetColumn("col2", nil)
		assert.NotNil(t, col2)
		col3 := queryInstance.GetColumn("col3", nil)
		assert.NotNil(t, col3)
		col4 := queryInstance.GetColumn("col4", nil)
		assert.NotNil(t, col4)
		col5 := queryInstance.GetColumn("col5", nil)
		assert.NotNil(t, col5)
		col11 := queryInstance.GetColumn("col11", nil)
		assert.Nil(t, col11)
	})
	t.Run("Explain", func(t *testing.T) {
		pgStatDatabase.Check()
		fmt.Println(pgStatDatabase.Explain())
	})
}
func TestQuery(t *testing.T) {
	query := &Query{}
	t.Run("Query_TimeoutDuration_other", func(t *testing.T) {
		r := query.TimeoutDuration()
		assert.Equal(t, time.Duration(float64(time.Second)*query.Timeout), r)
	})
	t.Run("IsPrimary", func(t *testing.T) {
		assert.Equal(t, true, query.IsPrimary())
		query.DbRole = "primary"
		assert.Equal(t, true, query.IsPrimary())
		assert.Equal(t, false, query.IsStandby())
		query.DbRole = "standby"
		assert.Equal(t, true, query.IsStandby())
		assert.Equal(t, false, query.IsPrimary())
		query.DbRole = "standby111"
		assert.Equal(t, false, query.IsStandby())
		assert.Equal(t, false, query.IsPrimary())
	})
}
