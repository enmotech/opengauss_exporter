// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"database/sql"
	"errors"
	"fmt"
	"gitee.com/opengauss/openGauss-connector-go-pq"
	"github.com/blang/semver"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/sirupsen/logrus"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	serverLabelName = "server"
	staticLabelName = "static"
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

// ServerWithDisableCache  will specify metric namespace, by default is pg or pgbouncer
func ServerWithDisableCache(b bool) ServerOpt {
	return func(s *Server) {
		s.disableCache = b
	}
}
func ServerWithTimeToString(b bool) ServerOpt {
	return func(s *Server) {
		s.timeToString = b
	}
}

func ServerWithParallel(i int) ServerOpt {
	return func(s *Server) {
		s.parallel = i
	}
}

type Server struct {
	fingerprint            string
	dsn                    string
	db                     *sql.DB
	labels                 prometheus.Labels
	primary                bool
	namespace              string // default prometheus namespace from cmd args
	disableSettingsMetrics bool
	notCollInternalMetrics bool // 不采集部分指标
	disableCache           bool
	timeToString           bool

	parallel int
	// Last version used to calculate metric map. If mismatch on scrape,
	// then maps are recalculated.
	lastMapVersion semver.Version
	// Currently active metric map
	queryInstanceMap map[string]*QueryInstance
	lock             sync.RWMutex
	// Currently cached metrics
	cacheMtx         sync.Mutex
	metricCache      map[string]*cachedMetrics
	UP               bool
	ScrapeTotalCount int64     // 采集指标个数
	ScrapeErrorCount int64     // 采集失败个数
	scrapeBegin      time.Time // server level scrape begin
	scrapeDone       time.Time // server last scrape done

	up               prometheus.Gauge
	recovery         prometheus.Gauge   // postgres is in recovery ?
	lastScrapeTime   prometheus.Gauge   // exporter level: last scrape timestamp
	scrapeDuration   prometheus.Gauge   // exporter level: seconds spend on scrape
	scrapeTotalCount prometheus.Counter // exporter level: total scrape count of this server
	scrapeErrorCount prometheus.Counter // exporter level: error scrape count

	queryCacheTTL          map[string]float64 // internal query metrics: cache time to live
	queryScrapeTotalCount  map[string]float64 // internal query metrics: total executed
	queryScrapeHitCount    map[string]float64 // internal query metrics: times serving from hit cache
	queryScrapeErrorCount  map[string]float64 // internal query metrics: times failed
	queryScrapeMetricCount map[string]float64 // internal query metrics: number of metrics scrapped
	queryScrapeDuration    map[string]float64 // internal query metrics: time spend on executing
}

// Close disconnects from OpenGauss.
func (s *Server) Close() error {
	if s.db == nil {
		return nil
	}
	s.UP = false

	return s.db.Close()
}

// Ping checks connection availability and possibly invalidates the connection if it fails.
func (s *Server) Ping() error {
	if err := s.db.Ping(); err != nil {
		if closeErr := s.Close(); closeErr != nil {
			log.Errorf("Error while closing non-pinging DB connection to %q: %v", s, closeErr)
		}
		return err
	}
	return nil
}

// String returns server's fingerprint.
func (s *Server) String() string {
	return s.labels[serverLabelName]
}

// Scrape loads metrics.
func (s *Server) Scrape(ch chan<- prometheus.Metric) error {
	if err := s.CheckConn(); err != nil {
		return err
	}

	s.lock.RLock()
	defer s.lock.RUnlock()
	if !s.notCollInternalMetrics {
		_ = s.setupServerInternalMetrics()
	}
	s.scrapeBegin = time.Now()

	var err error

	if !s.disableSettingsMetrics && !s.notCollInternalMetrics {
		if err = s.querySettings(ch); err != nil {
			err = fmt.Errorf("error retrieving settings: %s", err)
		}
	}

	errMap := s.queryMetrics(ch)
	if len(errMap) > 0 {
		err = fmt.Errorf("queryMetrics returned %d errors", len(errMap))
	}
	if !s.notCollInternalMetrics {
		s.scrapeDone = time.Now()
		// 最后采集时间
		s.lastScrapeTime.Set(float64(s.scrapeDone.Unix()))
		// 采集耗时
		s.scrapeDuration.Set(s.scrapeDone.Sub(s.scrapeBegin).Seconds())

		s.collectorServerInternalMetrics(ch)
	}

	return err
}

func (s *Server) setupServerInternalMetrics() error {

	s.scrapeTotalCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: s.namespace, ConstLabels: s.labels,
		Subsystem: "exporter_query", Name: "scrape_total_count", Help: "times exporter was scraped for metrics",
	})
	s.scrapeErrorCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: s.namespace, ConstLabels: s.labels,
		Subsystem: "exporter_query", Name: "scrape_error_count", Help: "times exporter was scraped for metrics and failed",
	})
	s.scrapeDuration = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: s.namespace, ConstLabels: s.labels,
		Subsystem: "exporter_query", Name: "scrape_duration", Help: "seconds exporter spending on scrapping",
	})
	s.lastScrapeTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: s.namespace, ConstLabels: s.labels,
		Subsystem: "exporter_query", Name: "last_scrape_time", Help: "seconds exporter spending on scrapping",
	})
	s.recovery = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: s.namespace, ConstLabels: s.labels,
		Name: "in_recovery", Help: "server is in recovery mode? 1 for yes 0 for no",
	})
	s.up = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: s.namespace, ConstLabels: s.labels,
		Name: "up", Help: "always be 1 if your could retrieve metrics",
	})
	return nil
}

func (s *Server) collectorServerInternalMetrics(ch chan<- prometheus.Metric) {
	if s.notCollInternalMetrics {
		return
	}
	if s.UP {
		s.up.Set(1)
		if s.primary {
			s.recovery.Set(0)
		} else {
			s.recovery.Set(1)
		}
	} else {
		s.up.Set(0)
		s.scrapeErrorCount.Add(1)
	}

	versionDesc := prometheus.NewDesc(fmt.Sprintf("%s_%s", s.namespace, "version"),
		"Version string as reported by OpenGauss", []string{"version", "short_version"}, s.labels)
	version := prometheus.MustNewConstMetric(versionDesc,
		prometheus.UntypedValue, 1, s.lastMapVersion.String(), s.lastMapVersion.String())
	s.scrapeTotalCount.Add(float64(s.ScrapeTotalCount))
	s.scrapeErrorCount.Add(float64(s.ScrapeErrorCount))

	ch <- s.up
	ch <- s.recovery
	ch <- s.scrapeTotalCount
	ch <- s.scrapeErrorCount
	ch <- s.scrapeDuration
	ch <- s.lastScrapeTime
	ch <- version

}
func (s *Server) CheckConn() error {
	if s.db == nil || !s.UP {
		return fmt.Errorf("not connect database")
	}
	return nil
}

// IsPrimary return true is primary database. false is standby database
func (s *Server) IsPrimary() (bool, error) {

	if err := s.CheckConn(); err != nil {
		return false, err
	}

	var b bool
	sqlText := "SELECT pg_is_in_recovery()"
	logrus.Debugf(sqlText)
	if err := s.db.QueryRow(sqlText).Scan(&b); err != nil {
		return false, err
	}
	return !b, nil
}

func (s *Server) DBRole() string {
	if s.primary {
		return "primary"
	}
	return "standby"
}

// 连接数据查询监控指标

func (s *Server) QueryDatabases() ([]string, error) {
	rows, err := s.db.Query(`SELECT datname FROM pg_database
	WHERE datallowconn = true
	AND datistemplate = false
	AND datname != current_database()`) // nolint: safesql
	if err != nil {
		return nil, fmt.Errorf("Error retrieving databases: %v", err)
	}
	defer rows.Close() // nolint: errcheck

	var databaseName string
	result := make([]string, 0)
	for rows.Next() {
		err = rows.Scan(&databaseName)
		if err != nil {
			return nil, errors.New(fmt.Sprintln("Error retrieving rows:", err))
		}
		result = append(result, databaseName)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *Server) getVersion() error {

	if err := s.CheckConn(); err != nil {
		return err
	}
	var versionString string
	err := s.db.QueryRow("SELECT version();").Scan(&versionString)
	if err != nil {
		return err
	}
	semanticVersion, err := parseVersionSem(versionString)
	if err != nil {
		return fmt.Errorf("Error parsing version string err %s ", err)
	}
	s.lastMapVersion = semanticVersion
	return nil
}
func (s *Server) ConnectDatabase() error {
	db, err := sql.Open("opengauss", s.dsn)
	s.db = db
	if err != nil {
		s.UP = false
		return err
	}

	if err = s.Ping(); err != nil {
		return err
	}
	s.db.SetConnMaxIdleTime(120 * time.Second)
	s.db.SetMaxIdleConns(s.parallel)
	s.db.SetMaxOpenConns(s.parallel)
	s.UP = true
	return nil
}

func NewServer(dsn string, opts ...ServerOpt) (*Server, error) {
	// 获取server名称 ip:port
	fingerprint, err := parseFingerprint(dsn)
	if err != nil {
		return nil, err
	}

	log.Infof("Established new database connection to %q.", fingerprint)

	s := &Server{
		fingerprint: fingerprint,
		dsn:         dsn,
		primary:     false,
		labels: prometheus.Labels{
			serverLabelName: fingerprint,
		},
		metricCache: make(map[string]*cachedMetrics),
	}

	for _, opt := range opts {
		opt(s)
	}

	if err = s.ConnectDatabase(); err != nil {
		return s, err
	}
	return s, nil
}

// Servers contains a collection of servers to OpenGauss.
type Servers struct {
	m       sync.Mutex
	servers map[string]*Server
	opts    []ServerOpt
}

// NewServers creates a collection of servers to OpenGauss.
func NewServers(opts ...ServerOpt) *Servers {
	return &Servers{
		servers: make(map[string]*Server),
		opts:    opts,
	}
}

// GetServer returns established connection from a collection.
func (s *Servers) GetServer(dsn string) (*Server, error) {
	s.m.Lock()
	defer s.m.Unlock()
	var err error
	var ok bool
	errCount := 0 // start at zero because we increment before doing work
	retries := 3
	var server *Server
	for {
		if errCount++; errCount > retries {
			return nil, err
		}
		server, ok = s.servers[dsn]
		if !ok {
			server, err = NewServer(dsn, s.opts...)
			if err != nil {
				log.Errorf("new server %s err %s", server.fingerprint, err)
				time.Sleep(time.Duration(errCount) * time.Second)
				continue
			}
			s.servers[dsn] = server
		}
		if !server.UP {
			if err = server.ConnectDatabase(); err != nil {
				log.Errorf("new server %s err %s", server.fingerprint, err)
				time.Sleep(time.Duration(errCount) * time.Second)
				continue
			}
		}
		if err = server.Ping(); err != nil {
			// delete(s.servers, dsn)
			log.Errorf("ping %s err %s", server.fingerprint, err)
			time.Sleep(time.Duration(errCount) * time.Second)
			continue
		}
		break
	}
	isPrimary, err := server.IsPrimary()
	if err != nil {
		// log.Errorf("Error querying IsPrimary (%s): %v", ShadowDSN(dsn), err)
		return nil, err
	}
	// If autoDiscoverDatabases is true, set first dsn as primary database (Default: false)
	server.primary = isPrimary
	// server.primary = false

	if err = server.getVersion(); err != nil {
		return nil, err
	}

	return server, nil
}

// Close disconnects from all known servers.
func (s *Servers) Close() {
	s.m.Lock()
	defer s.m.Unlock()
	for _, server := range s.servers {
		if err := server.Close(); err != nil {
			log.Errorf("failed to close connection to %q: %v", server, err)
		}
	}
}

// Convert database.sql types to float64s for Prometheus consumption. Null types are mapped to NaN. string and []byte
// types are mapped as NaN and !ok
func dbToFloat64(t interface{}) (float64, bool) {
	switch v := t.(type) {
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case time.Time:
		return float64(v.Unix()), true
	case []byte:
		// Try and convert to string and then parse to a float64
		strV := string(v)
		result, err := strconv.ParseFloat(strV, 64)
		if err != nil {
			log.Infoln("Could not parse []byte:", err)
			return math.NaN(), false
		}
		return result, true
	case string:
		result, err := strconv.ParseFloat(v, 64)
		if err != nil {
			log.Infoln("Could not parse string:", err)
			return math.NaN(), false
		}
		return result, true
	case bool:
		if v {
			return 1.0, true
		}
		return 0.0, true
	case nil:
		return math.NaN(), true
	default:
		return math.NaN(), false
	}
}

// Convert database.sql to string for Prometheus labels. Null types are mapped to empty strings.
func dbToString(t interface{}, time2string bool) (string, bool) {
	switch v := t.(type) {
	case int64:
		return fmt.Sprintf("%v", v), true
	case float64:
		return fmt.Sprintf("%v", v), true
	case time.Time:
		if time2string {
			return v.Format(time.RFC3339Nano), true
		}
		return fmt.Sprintf("%v%03d", v.Unix(), v.Nanosecond()/1000000), true
	case nil:
		return "", true
	case []byte:
		// Try and convert to string
		return string(v), true
	case string:
		return v, true
	case bool:
		if v {
			return "true", true
		}
		return "false", true
	default:
		return "", false
	}
}

func parseFingerprint(url string) (string, error) {
	dsn, err := pq.ParseURL(url)
	if err != nil {
		dsn = url
	}

	pairs := strings.Split(dsn, " ")
	kv := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		split := strings.SplitN(pair, "=", 2)
		if len(split) != 2 {
			return "", fmt.Errorf("malformed dsn %q", dsn)
		}
		kv[split[0]] = split[1]
	}

	var fingerprint string

	if host, ok := kv["host"]; ok {
		fingerprint += host
	} else {
		fingerprint += "localhost"
	}

	if port, ok := kv["port"]; ok {
		fingerprint += ":" + port
	} else {
		fingerprint += ":5432"
	}

	return fingerprint, nil
}
