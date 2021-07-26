// Copyright © 2021 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v2"
	"strings"
)

// setupInternalMetrics setup Internal Metrics
func (e *Exporter) setupInternalMetrics() {

	e.configFileError = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   e.namespace,
		Subsystem:   "exporter",
		Name:        "use_config_load_error",
		Help:        "Whether the user config file was loaded and parsed successfully (1 for error, 0 for success).",
		ConstLabels: e.constantLabels,
	}, []string{"filename", "hashsum"})
	// exporter level metrics
	e.exporterUp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: e.namespace, ConstLabels: e.constantLabels,
		Subsystem: "exporter", Name: "up", Help: "always be 1 if your could retrieve metrics",
	})
	e.exporterUptime = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: e.namespace, ConstLabels: e.constantLabels,
		Subsystem: "exporter", Name: "uptime", Help: "seconds since exporter primary server inited",
	})
	e.scrapeTotalCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: e.namespace, ConstLabels: e.constantLabels,
		Subsystem: "exporter", Name: "scrape_total_count", Help: "times exporter was scraped for metrics",
	})
	e.scrapeErrorCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: e.namespace, ConstLabels: e.constantLabels,
		Subsystem: "exporter", Name: "scrape_error_count", Help: "times exporter was scraped for metrics and failed",
	})
	e.scrapeDuration = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: e.namespace, ConstLabels: e.constantLabels,
		Subsystem: "exporter", Name: "scrape_duration", Help: "seconds exporter spending on scrapping",
	})
	e.lastScrapeTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: e.namespace, ConstLabels: e.constantLabels,
		Subsystem: "exporter", Name: "last_scrape_time", Help: "seconds exporter spending on scrapping",
	})
}

// GetMetricsList Get Metrics List
func (e *Exporter) GetMetricsList() map[string]*QueryInstance {
	if e.allMetricMap == nil {
		return nil
	}
	return e.allMetricMap
}

func (e *Exporter) PrintMetricsList() (string, error) {
	if e.allMetricMap == nil {
		return "", nil
	}
	var metricList []string
	for _, q := range e.allMetricMap {
		metric := q.Explain()
		metricList = append(metricList, metric)
	}
	return strings.Join(metricList, "\n\n"), nil
}
func (e *Exporter) PrintMetricsList1() (string, error) {
	if e.allMetricMap == nil {
		return "", nil
	}
	buffer, err := yaml.Marshal(e.allMetricMap)
	if err != nil {
		return "", err
	}
	return string(buffer), err
}
