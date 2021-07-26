// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"strings"
	"sync"
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
	allMetricMap           map[string]*QueryInstance // 全部采集指标 不判断Public为true
	priMetricMap           map[string]*QueryInstance // 私有采集指标 autoDiscover下公用指标,只采集一次
	collStatus             map[string]bool
	constantLabels         prometheus.Labels // 用户定义标签

	lock sync.RWMutex // export lock

	scrapeBegin time.Time // server level scrape begin
	scrapeDone  time.Time // server last scrape done
	exportInit  time.Time // server init timestamp

	configFileError  *prometheus.GaugeVec // 读取配置文件失败采集
	exporterUp       prometheus.Gauge     // exporter level: always set ot 1
	exporterUptime   prometheus.Gauge     // exporter level: primary target server uptime (exporter itself)
	lastScrapeTime   prometheus.Gauge     // exporter level: last scrape timestamp
	scrapeDuration   prometheus.Gauge     // exporter level: seconds spend on scrape
	scrapeTotalCount prometheus.Counter   // exporter level: total scrape count of this server
	scrapeErrorCount prometheus.Counter   // exporter level: error scrape count

	timeToString bool
	parallel     int
}

// NewExporter New Exporter
func NewExporter(opts ...Opt) (e *Exporter, err error) {
	e = &Exporter{
		allMetricMap: defaultMonList, // default metric
		priMetricMap: map[string]*QueryInstance{},
		parallel:     1,
		exportInit:   time.Now(),
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

	if e.parallel == 0 {
		e.parallel = 1
	}
	return e, nil
}

// initDefaultMetric init default metric
func (e *Exporter) initDefaultMetric() {
	for _, q := range e.allMetricMap {
		_ = q.Check()
	}
}

// loadConfig Load the configuration file, the same indicator in the configuration file overwrites the default configuration
// 加载配置文件,配置文件里相同指标覆盖默认配置
func (e *Exporter) loadConfig() error {
	if e.configPath == "" {
		return nil
	}
	queryMap, err := LoadConfig(e.configPath)
	if err != nil {
		return err
	}
	for name, query := range queryMap {
		var found, found1 bool
		for defName, defQuery := range e.allMetricMap {
			if strings.EqualFold(defQuery.Name, query.Name) {
				e.allMetricMap[defName] = query
				found = true
				break
			}
		}
		if !found {
			e.allMetricMap[name] = query
		}
		// 如果是通用指标不判断私有
		if query.Public {
			continue
		}
		for defName, defQuery := range e.priMetricMap {
			if strings.EqualFold(defQuery.Name, query.Name) {
				e.priMetricMap[defName] = query
				found1 = true
				break
			}
		}
		if !found1 {
			e.priMetricMap[name] = query
		}
	}
	return nil
}

func (e *Exporter) setupServers() {
	e.servers = NewServers(ServerWithLabels(e.constantLabels),
		ServerWithNamespace(e.namespace),
		ServerWithDisableSettingsMetrics(e.disableSettingsMetrics),
		ServerWithDisableCache(e.disableCache),
		ServerWithTimeToString(e.timeToString),
		ServerWithParallel(e.parallel),
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
	e.collectServerMetrics()
	e.collectInternalMetrics(ch)
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	e.lock.Lock()
	defer e.lock.Unlock()
	// 设置采集开始时间
	e.scrapeBegin = time.Now()

	dsnList := e.dsn
	if e.autoDiscovery {
		dsnList = e.discoverDatabaseDSNs()
	}

	var errorsCount int
	var connectionErrorsCount int
	// 重置采集状态
	e.collStatus = map[string]bool{}
	for _, dsn := range dsnList {
		// log.Debugf(dsn)
		if err := e.scrapeDSN(ch, dsn); err != nil {
			errorsCount++

			log.Errorf(err.Error())

			if _, ok := err.(*ErrorConnectToServer); ok {
				connectionErrorsCount++
			}
		}
	}
	// 设置结束开始时间
	e.scrapeDone = time.Now()
	// 最后采集时间
	e.lastScrapeTime.Set(float64(e.scrapeDone.Unix()))
	// 采集耗时
	e.scrapeDuration.Set(e.scrapeDone.Sub(e.scrapeBegin).Seconds())
	// 在线时间
	e.exporterUptime.Set(time.Now().Sub(e.exportInit).Seconds())
	// 在线
	e.exporterUp.Set(1)
	log.Debugf("the errorsCount %v ", errorsCount)
}
func (e *Exporter) collectServerMetrics() {
	for _, s := range e.servers.servers {
		e.scrapeTotalCount.Add(float64(s.ScrapeTotalCount))
		e.scrapeErrorCount.Add(float64(s.ScrapeErrorCount))
	}
}
func (e *Exporter) collectInternalMetrics(ch chan<- prometheus.Metric) {

	ch <- e.exporterUp
	ch <- e.exporterUptime
	ch <- e.lastScrapeTime
	ch <- e.scrapeTotalCount
	ch <- e.scrapeErrorCount
	ch <- e.scrapeDuration

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

	_, ok := e.collStatus[server.fingerprint]
	// 如果同一个ip+端口采集过一次,说明公共指标已采集,不需要在采集了
	if ok {
		server.queryInstanceMap = e.priMetricMap
		server.notCollInternalMetrics = true
	} else {
		server.queryInstanceMap = e.allMetricMap
		defer func() { e.collStatus[server.fingerprint] = true }()
	}

	return server.Scrape(ch)
}
func (e *Exporter) Close() {
	e.servers.Close()
}
