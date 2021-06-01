// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_Exporter(t *testing.T) {
	exporter, err := NewExporter(
		WithParallel(2),
		WithConfig("../../og_exporter_default.yaml"),
	)
	if err != nil {
		t.Error(err)
		return
	}
	t.Run("initDefaultMetric", func(t *testing.T) {
		exporter.initDefaultMetric()
	})
	t.Run("LoadConfig", func(t *testing.T) {
		exporter.configPath = "a1.yaml"
		err := exporter.loadConfig()
		assert.Error(t, err)
	})
	t.Run("GetMetricsList", func(t *testing.T) {
		list := exporter.GetMetricsList()
		assert.NotNil(t, list)
	})
	t.Run("LoadConfig_configPath_null", func(t *testing.T) {
		exporter.configPath = ""
		err := exporter.loadConfig()
		assert.NoError(t, err)
	})
	t.Run("Describe", func(t *testing.T) {
		ch := make(chan *prometheus.Desc, 100)
		exporter.Describe(ch)
		close(ch)
	})
	t.Run("Collect", func(t *testing.T) {
		ch := make(chan prometheus.Metric, 100)
		exporter.Collect(ch)
		close(ch)
	})
	// t.Run("Close", func(t *testing.T) {
	// 	exporter.Check()
	// })
	t.Run("Close", func(t *testing.T) {
		exporter.Close()
	})

}
