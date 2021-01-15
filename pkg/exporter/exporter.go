// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"strings"
	"time"
)

type Exporter struct {
	dsn                    []string
	configPath             string   // config file path /directory
	disableCache           bool     // always execute query when been scrapped
	autoDiscovery          bool     // discovery other database on primary server
	failFast               bool     // fail fast instead fof waiting during start-up ?
	excludedDatabases      []string // excluded database for auto discovery
	disableSettingsMetrics bool
	tags                   []string
	namespace              string
	servers                *Servers
	metricMap              map[string]*QueryInstance

	constantLabels  prometheus.Labels    // 用户定义标签
	duration        prometheus.Gauge     // 采集时间
	error           prometheus.Gauge     // 采集指标时错误统计
	up              prometheus.Gauge     //
	configFileError *prometheus.GaugeVec // 读取配置文件失败采集
	totalScrapes    prometheus.Counter   // 采集次数
	timeToString    bool
}

// NewExporter New Exporter
func NewExporter(opts ...Opt) (e *Exporter, err error) {
	e = &Exporter{
		metricMap: defaultMonList, // default metric
	}
	for _, opt := range opts {
		opt(e)
	}

	e.initDefaultMetric()

	if err := e.loadConfig(); err != nil {
		return nil, err
	}
	e.setupInternalMetrics()
	e.setupServers()
	return e, nil
}

// initDefaultMetric init default metric
func (e *Exporter) initDefaultMetric() {
	for _, q := range e.metricMap {
		_ = q.Check()
	}
}

// loadConfig Load the configuration file, the same indicator in the configuration file overwrites the default configuration
// 加载配置文件,配置文件里相同指标覆盖默认配置
func (e *Exporter) loadConfig() error {
	if e.configPath == "" {
		return nil
	}
	queryList, err := LoadConfig(e.configPath)
	if err != nil {
		return err
	}
	for name, query := range queryList {
		var found bool
		for defName, defQuery := range e.metricMap {
			if strings.EqualFold(defQuery.Name, query.Name) {
				e.metricMap[defName] = query
				found = true
				break
			}
		}
		if !found {
			e.metricMap[name] = query
		}
	}
	return nil
}

// GetMetricsList Get Metrics List
func (e *Exporter) GetMetricsList() map[string]*QueryInstance {
	if e.metricMap == nil {
		return nil
	}
	return e.metricMap
}

// setupInternalMetrics setup Internal Metrics
func (e *Exporter) setupInternalMetrics() {

	e.duration = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   e.namespace,
		Subsystem:   "exporter",
		Name:        "last_scrape_duration_seconds",
		Help:        "Duration of the last scrape of metrics from OpenGauss.",
		ConstLabels: e.constantLabels,
	})
	e.totalScrapes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   e.namespace,
		Subsystem:   "exporter",
		Name:        "scrapes_total",
		Help:        "Total number of times OpenGauss was scraped for metrics.",
		ConstLabels: e.constantLabels,
	})
	// 采集指标错误
	e.error = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   e.namespace,
		Subsystem:   "exporter",
		Name:        "last_scrape_error",
		Help:        "Whether the last scrape of metrics from OpenGauss resulted in an error (1 for error, 0 for success).",
		ConstLabels: e.constantLabels,
	})
	e.up = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   e.namespace,
		Name:        "up",
		Help:        "Whether the last scrape of metrics from OpenGauss was able to connect to the server (1 for yes, 0 for no).",
		ConstLabels: e.constantLabels,
	})
	e.configFileError = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   e.namespace,
		Subsystem:   "exporter",
		Name:        "use_config_load_error",
		Help:        "Whether the user config file was loaded and parsed successfully (1 for error, 0 for success).",
		ConstLabels: e.constantLabels,
	}, []string{"filename", "hashsum"})
}

func (e *Exporter) setupServers() {
	e.servers = NewServers(ServerWithLabels(e.constantLabels),
		ServerWithNamespace(e.namespace),
		ServerWithDisableSettingsMetrics(e.disableSettingsMetrics),
		ServerWithDisableCache(e.disableCache),
		ServerWithTimeToString(e.timeToString),
	)
}

// Describe implement prometheus.Collector
// -> Collect
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	metricCh := make(chan prometheus.Metric)
	doneCh := make(chan struct{})

	go func() {
		for m := range metricCh {
			ch <- m.Desc()
		}
		close(doneCh)
	}()

	e.Collect(metricCh)
	close(metricCh)
	<-doneCh
}

// Collect
// Collect->
// 		scrape->
//			-> discoverDatabaseDSNs
//			-> scrapeDSN
//				-> GetServer
// 				-> checkMapVersions
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.scrape(ch)

	ch <- e.duration
	ch <- e.totalScrapes
	ch <- e.error
	ch <- e.up
	e.configFileError.Collect(ch)
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	// 设置采集持续时间指标
	defer func(begun time.Time) {
		e.duration.Set(time.Since(begun).Seconds())
	}(time.Now())

	e.totalScrapes.Inc()

	dsnList := e.dsn
	if e.autoDiscovery {
		dsnList = e.discoverDatabaseDSNs()
	}

	var errorsCount int
	var connectionErrorsCount int

	for _, dsn := range dsnList {
		log.Debugf(dsn)
		if err := e.scrapeDSN(ch, dsn); err != nil {
			errorsCount++

			log.Errorf(err.Error())

			if _, ok := err.(*ErrorConnectToServer); ok {
				connectionErrorsCount++
			}
		}
	}

	switch {
	case connectionErrorsCount >= len(dsnList):
		e.up.Set(0)
	default:
		e.up.Set(1) // Didn't fail, can mark connection as up for this scrape.
	}
	log.Debugf("the errorsCount %v ", errorsCount)
	switch errorsCount {
	case 0:
		e.error.Set(0)
	default:
		e.error.Set(1)
	}
}

func (e *Exporter) discoverDatabaseDSNs() []string {
	result := []string{}
	for _, dsn := range e.dsn {
		parsedDSN, err := parseDsn(dsn)
		if err != nil {
			log.Errorf("Unable to parse DSN (%s): %v", ShadowDSN(dsn), err)
			continue
		}
		server, err := e.servers.GetServer(dsn)
		if err != nil {
			log.Errorf("Error opening connection to database (%s): %v", ShadowDSN(dsn), err)
			continue
		}

		// If autoDiscoverDatabases is true, set first dsn as master database (Default: false)
		server.master = true

		databaseNames, err := server.QueryDatabases()
		if err != nil {
			log.Errorf("Error querying databases (%s): %v", ShadowDSN(dsn), err)
			continue
		}
		result = append(result, genDSNString(parsedDSN))
		for _, databaseName := range databaseNames {
			if Contains(e.excludedDatabases, databaseName) {
				continue
			}
			parsedDSN["database"] = databaseName
			result = append(result, genDSNString(parsedDSN))
		}
	}
	return result
}

func (e *Exporter) scrapeDSN(ch chan<- prometheus.Metric, dsn string) error {
	server, err := e.servers.GetServer(dsn)

	if err != nil {
		return &ErrorConnectToServer{fmt.Sprintf("Error opening connection to database (%s): %s", ShadowDSN(dsn), err.Error())}
	}

	// Check if autoDiscoverDatabases is false, set dsn as master database (Default: false)
	if !e.autoDiscovery {
		server.master = true
	}

	// Check if map versions need to be updated
	if err := e.checkMapVersions(ch, server); err != nil {
		log.Warnln("Proceeding with outdated query maps, as the OpenGauss version could not be determined:", err)
	}

	return server.Scrape(ch)
}

func (e *Exporter) checkMapVersions(ch chan<- prometheus.Metric, server *Server) error {
	log.Debugf("Querying OpenGauss Version on %q", server)
	versionRow := server.db.QueryRow("SELECT version();")
	var versionString string
	err := versionRow.Scan(&versionString)
	if err != nil {
		return fmt.Errorf("Error scanning version string on %q: %v ", server, err)
	}
	semanticVersion, err := parseVersionSem(versionString)
	if err != nil {
		return fmt.Errorf("Error parsing version string on %q: %v ", server, err)
	}
	// Check if semantic version changed and recalculate maps if needed.
	if semanticVersion.NE(server.lastMapVersion) || server.queryInstanceMap == nil {
		log.Infof("Semantic Version Changed on %s: %s -> %s", server, server.lastMapVersion, semanticVersion)
		server.mappingMtx.Lock()
		server.queryInstanceMap = e.metricMap
		server.lastMapVersion = semanticVersion
		server.mappingMtx.Unlock()

	}

	versionDesc := prometheus.NewDesc(fmt.Sprintf("%s_%s", e.namespace, staticLabelName),
		"Version string as reported by OpenGauss", []string{"version", "short_version"}, server.labels)

	if server.master {
		ch <- prometheus.MustNewConstMetric(versionDesc,
			prometheus.UntypedValue, 1, parseVersion(versionString), semanticVersion.String())
	}
	return nil
}

func (e *Exporter) Check() error {
	return nil
}

func (e *Exporter) Close() {
	e.servers.Close()
}
