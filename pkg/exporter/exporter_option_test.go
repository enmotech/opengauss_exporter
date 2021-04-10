// Copyright © 2021 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExporter_Opt(t *testing.T) {
	exporter := &Exporter{}
	t.Run("WithDNS", func(t *testing.T) {
		WithDNS([]string{"a1"})(exporter)
		assert.Equal(t, []string{"a1"}, exporter.dsn)
	})
	t.Run("WithConfig", func(t *testing.T) {
		WithConfig("a1")(exporter)
		assert.Equal(t, "a1", exporter.configPath)
	})
	t.Run("WithConstLabels", func(t *testing.T) {
		label := "a1=1,a2=2"
		WithConstLabels(label)(exporter)
		assert.Equal(t, prometheus.Labels{"a1": "1", "a2": "2"}, exporter.constantLabels)
	})
	t.Run("WithCacheDisabled", func(t *testing.T) {
		WithCacheDisabled(false)(exporter)
		assert.Equal(t, false, exporter.disableCache)
	})
	t.Run("WithDisableSettingsMetrics", func(t *testing.T) {
		WithDisableSettingsMetrics(false)(exporter)
		assert.Equal(t, false, exporter.disableSettingsMetrics)
	})
	t.Run("WithFailFast", func(t *testing.T) {
		WithFailFast(false)(exporter)
		assert.Equal(t, false, exporter.failFast)
	})
	t.Run("WithNamespace", func(t *testing.T) {
		WithNamespace("a1")(exporter)
		assert.Equal(t, "a1", exporter.namespace)
	})
	t.Run("WithTags", func(t *testing.T) {
		label := "a1=1,a2=2"
		WithTags(label)(exporter)
		assert.Equal(t, []string{"a1=1", "a2=2"}, exporter.tags)
	})
	t.Run("WithTimeToString", func(t *testing.T) {
		WithTimeToString(false)(exporter)
		assert.Equal(t, false, exporter.timeToString)
	})
	t.Run("WithParallel", func(t *testing.T) {
		WithParallel(5)(exporter)
		assert.Equal(t, 5, exporter.parallel)
	})
	t.Run("WithAutoDiscovery", func(t *testing.T) {
		WithAutoDiscovery(false)(exporter)
		assert.Equal(t, false, exporter.autoDiscovery)
	})
	t.Run("WithExcludeDatabases", func(t *testing.T) {
		WithExcludeDatabases("a1,a2")(exporter)
		assert.Equal(t, []string{"a1", "a2"}, exporter.excludedDatabases)
	})
}
