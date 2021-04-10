// Copyright © 2021 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

type cachedMetrics struct {
	metrics        []prometheus.Metric
	lastScrape     time.Time
	nonFatalErrors []error
	err            error
	name           string
	collect        bool
}

// IsValid true is cache valid
func (c *cachedMetrics) IsValid(ttl float64) bool {
	if ttl == 0 {
		return false
	}
	return !(time.Now().Sub(c.lastScrape).Seconds() >= ttl)
}

func (c *cachedMetrics) IsCollect() bool {
	return c.collect
}
