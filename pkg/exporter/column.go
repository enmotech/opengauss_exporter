// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	DISCARD      = "DISCARD" // Ignore this column (when SELECT *)
	LABEL        = "LABEL"   // Use this column as a label
	COUNTER      = "COUNTER" // Use this column as a counter
	GAUGE        = "GAUGE"   // Use this column as a gauge
	HISTOGRAM    = "HISTOGRAM"
	MappedMETRIC = "MAPPEDMETRIC"
	DURATION     = "DURATION"
)

var ColumnUsage = map[string]bool{
	DISCARD:      true,
	LABEL:        true,
	COUNTER:      true,
	GAUGE:        true,
	HISTOGRAM:    true,
	MappedMETRIC: true,
	DURATION:     true,
}

type Column struct {
	Name           string               `yaml:"name"`
	Desc           string               `yaml:"description,omitempty"`
	Usage          string               `yaml:"usage,omitempty"`
	Rename         string               `yaml:"rename,omitempty"`
	DisCard        bool                 `yaml:"-"`
	Histogram      bool                 `yaml:"-"` // Should metric be treated as a histogram?
	PrometheusDesc *prometheus.Desc     `yaml:"-"`
	PrometheusType prometheus.ValueType `yaml:"-"`
}

func (c *Column) String() string {
	return fmt.Sprintf("%-8s %-30s %s", c.Usage, c.Name, c.Desc)
}
