// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"strings"
)

const (
	DISCARD = "DISCARD" // Ignore this column (when SELECT *)
	LABEL   = "LABEL"   // Use this column as a label
	COUNTER = "COUNTER" // Use this column as a counter
	GAUGE   = "GAUGE"   // Use this column as a gauge
)

var ColumnUsage = map[string]bool{
	DISCARD: true,
	LABEL:   true,
	COUNTER: true,
	GAUGE:   true,
}

type Column struct {
	Name           string               `yaml:"name"`
	Desc           string               `yaml:"description,omitempty"`
	Usage          string               `yaml:"usage,omitempty"`
	Rename         string               `yaml:"rename,omitempty"`
	DisCard        bool                 `yaml:"-"`
	PrometheusDesc *prometheus.Desc     `yaml:"-"`
	PrometheusType prometheus.ValueType `yaml:"-"`
}

// PrometheusValueType returns column's corresponding prometheus value type
func (c *Column) PrometheusValueType() prometheus.ValueType {
	switch strings.ToUpper(c.Usage) {
	case GAUGE:
		return prometheus.GaugeValue
	case COUNTER:
		return prometheus.CounterValue
	default:
		// it's user's responsibility to make sure this is a value column
		panic(fmt.Errorf("column %s does not have a valid value type %s", c.Name, c.Usage))
	}
}

// String turns column into a one-line text representation
func (c *Column) String() string {
	return fmt.Sprintf("%-8s %-20s %s", c.Usage, c.Name, c.Desc)
}
