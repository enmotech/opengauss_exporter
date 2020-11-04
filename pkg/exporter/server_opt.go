// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"github.com/prometheus/client_golang/prometheus"
)

// ServerOpt configures a server.
type ServerOpt func(*Server)

// ServerWithLabels configures a set of labels.
func ServerWithLabels(labels prometheus.Labels) ServerOpt {
	return func(s *Server) {
		for k, v := range labels {
			s.labels[k] = v
		}
	}
}

// ServerWithNamespace will specify metric namespace, by default is pg or pgbouncer
func ServerWithNamespace(namespace string) ServerOpt {
	return func(s *Server) {
		s.namespace = namespace
	}
}

// ServerWithDisableSettingsMetrics will specify metric namespace, by default is pg or pgbouncer
func ServerWithDisableSettingsMetrics(b bool) ServerOpt {
	return func(s *Server) {
		s.disableSettingsMetrics = b
	}
}

// ServerWithDisableSettingsMetrics will specify metric namespace, by default is pg or pgbouncer
func ServerWithDisableCache(b bool) ServerOpt {
	return func(s *Server) {
		s.disableCache = b
	}
}
