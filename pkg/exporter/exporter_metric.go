// Copyright © 2021 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"gopkg.in/yaml.v2"
	"strings"
)

// GetMetricsList Get Metrics List
func (e *Exporter) GetMetricsList() map[string]*QueryInstance {
	if e.metricMap == nil {
		return nil
	}
	return e.metricMap
}

func (e *Exporter) PrintMetricsList() (string, error) {
	if e.metricMap == nil {
		return "", nil
	}
	var metricList []string
	for _, q := range e.metricMap {
		metric := q.Explain()
		metricList = append(metricList, metric)
	}
	return strings.Join(metricList, "\n\n"), nil
}
func (e *Exporter) PrintMetricsList1() (string, error) {
	if e.metricMap == nil {
		return "", nil
	}
	buffer, err := yaml.Marshal(e.metricMap)
	if err != nil {
		return "", err
	}
	return string(buffer), err
}
