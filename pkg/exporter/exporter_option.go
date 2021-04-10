// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"strings"
)

// Opt ExporterOpt configures Exporter
type Opt func(*Exporter)

// WithDNS add config dsn to Exporter
func WithDNS(dsn []string) Opt {
	return func(e *Exporter) {
		e.dsn = dsn
	}
}

// WithConfig add config path to Exporter
func WithConfig(configPath string) Opt {
	return func(e *Exporter) {
		e.configPath = configPath
	}
}

// WithConstLabels add const label to exporter. 0 length label returns nil
func WithConstLabels(s string) Opt {
	return func(e *Exporter) {
		e.constantLabels = parseConstLabels(s)
	}
}

// WithCacheDisabled set cache param to exporter
func WithCacheDisabled(disableCache bool) Opt {
	return func(e *Exporter) {
		e.disableCache = disableCache
	}
}

// WithDisableSettingsMetrics set cache param to exporter
func WithDisableSettingsMetrics(b bool) Opt {
	return func(e *Exporter) {
		e.disableSettingsMetrics = b
	}
}

// WithFailFast marks exporter fail instead of waiting during start-up
func WithFailFast(failFast bool) Opt {
	return func(e *Exporter) {
		e.failFast = failFast
	}
}

// WithNamespace will specify metric namespace, by default is pg or pgbouncer
func WithNamespace(namespace string) Opt {
	return func(e *Exporter) {
		e.namespace = namespace
	}
}

// WithTags will register given tags to Exporter and all belonged servers
func WithTags(tags string) Opt {
	return func(e *Exporter) {
		e.tags = parseCSV(tags)
	}
}

// WithTimeToString WithTags will register given tags to Exporter and all belonged servers
func WithTimeToString(b bool) Opt {
	return func(e *Exporter) {
		e.timeToString = b
	}
}
func WithParallel(i int) Opt {
	return func(e *Exporter) {
		e.parallel = i
	}
}

// WithAutoDiscovery configures exporter with excluded database
func WithAutoDiscovery(flag bool) Opt {
	return func(e *Exporter) {
		e.autoDiscovery = flag
	}
}

// WithExcludeDatabases configures exporter with excluded database
func WithExcludeDatabases(excludeStr string) Opt {
	return func(e *Exporter) {
		e.excludedDatabases = strings.Split(excludeStr, ",")
	}
}
