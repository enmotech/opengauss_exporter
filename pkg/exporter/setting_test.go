// Copyright © 2021 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestServer_querySettings(t *testing.T) {
	var (
		s  = &Server{}
		ch = make(chan prometheus.Metric, 100)
	)
	defer close(ch)
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Error(err)
		return
	}
	s.db = db
	t.Run("querySettings", func(t *testing.T) {
		mock.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows([]string{"name", "setting", "coalesce", "short_desc", "vartype"}).AddRow(
				"bool_off", "off", "", "Used to.", "bool").AddRow(
				"bool_on", "on", "", "Used to.", "bool").AddRow(
				"alarm_component", "/opt/snas/bin/snas_cm_cmd", "", "Used to.", "string").AddRow(
				"real", "1", "", "real.", "real").AddRow(
				"integer_ms", "500000", "ms", "Used to.", "integer").AddRow(
				"integer_min", "500000", "min", "Used to.", "integer").AddRow(
				"integer_h", "500000", "h", "Used to.", "integer").AddRow(
				"integer_d", "500000", "d", "Used to.", "integer").AddRow(
				"integer_kB", "500000", "kB", "Used to.", "integer").AddRow(
				"integer_MB", "500000", "MB", "Used to.", "integer").AddRow(
				"integer_GB", "500000", "GB", "Used to.", "integer").AddRow(
				"integer_TB", "500000", "TB", "Used to.", "integer").AddRow(
				"integer_8kB", "500000", "8kB", "Used to.", "integer").AddRow(
				"integer_16kB", "500000", "16kB", "Used to.", "integer").AddRow(
				"integer_32kB", "500000", "32kB", "Used to.", "integer").AddRow(
				"integer_16MB", "500000", "16MB", "Used to.", "integer").AddRow(
				"integer_32MB", "500000", "32MB", "Used to.", "integer").AddRow(
				"integer_64MB", "5000000", "64MB", "Used to.", "integer"))
		err := s.querySettings(ch)
		assert.NoError(t, err)
	})
	t.Run("querySettings", func(t *testing.T) {
		mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("a1"))
		err := s.querySettings(ch)
		assert.Error(t, err)
	})
	t.Run("querySettings", func(t *testing.T) {
		mock.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows([]string{"name", "setting", "coalesce", "short_desc", "vartype"}).AddRow(
				"bool_off", "off", "", "Used to.", "bool").AddRow(
				"bool_off", "off", "", "Used to.", "bool").RowError(1, fmt.Errorf("error")))
		err := s.querySettings(ch)
		assert.Error(t, err)
	})
	t.Run("querySettings", func(t *testing.T) {
		mock.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows([]string{"name", "setting", "coalesce", "short_desc", "vartype"}).AddRow(
				nil, "off", "", "Used to.", "bool"))
		err := s.querySettings(ch)
		assert.Error(t, err)
	})
	t.Run("normaliseUnit", func(t *testing.T) {
		pgSetting := &pgSetting{
			varType: "a1",
		}
		metric := pgSetting.metric("a1", nil)
		assert.Nil(t, metric)
	})
	t.Run("normaliseUnit", func(t *testing.T) {
		pgSetting := &pgSetting{
			varType: "a1",
			unit:    "ms",
			setting: "-1",
		}
		val, unit, err := pgSetting.normaliseUnit()
		assert.Equal(t, float64(-1), val)
		assert.Equal(t, "seconds", unit)
		assert.NoError(t, err)
	})
	t.Run("normaliseUnit_Unknown_unit", func(t *testing.T) {
		pgSetting := &pgSetting{
			varType: "a1",
			unit:    "ms1",
			setting: "-1",
		}
		val, unit, err := pgSetting.normaliseUnit()
		assert.Equal(t, float64(-1), val)
		assert.Equal(t, "", unit)
		assert.Error(t, err)
	})
	t.Run("normaliseUnit_ParseFloatunit", func(t *testing.T) {
		pgSetting := &pgSetting{
			varType: "a1",
			unit:    "ms1",
			setting: "a",
		}
		val, unit, err := pgSetting.normaliseUnit()
		assert.Equal(t, float64(0), val)
		assert.Equal(t, "", unit)
		assert.Error(t, err)
	})
}
