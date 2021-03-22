// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/blang/semver"
	"github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
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

// ServerWithDisableSettingsMetrics will specify metric namespace, by default is pg or pgbouncer
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
	dsn                    string
	db                     *sql.DB
	labels                 prometheus.Labels
	master                 bool
	namespace              string // default prometheus namespace from cmd args
	disableSettingsMetrics bool
	disableCache           bool
	timeToString           bool
	parallel               int
	// Last version used to calculate metric map. If mismatch on scrape,
	// then maps are recalculated.
	lastMapVersion semver.Version
	// Currently active metric map
	queryInstanceMap map[string]*QueryInstance
	mappingMtx       sync.RWMutex
	// Currently cached metrics
	metricCache map[string]*cachedMetrics
	cacheMtx    sync.Mutex
}

// Close disconnects from OpenGauss.
func (s *Server) Close() error {
	if s.db == nil {
		return nil
	}
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
	s.mappingMtx.RLock()
	defer s.mappingMtx.RUnlock()

	var err error

	if !s.disableSettingsMetrics && s.master {
		if err = s.querySettings(ch); err != nil {
			err = fmt.Errorf("error retrieving settings: %s", err)
		}
	}

	errMap := s.queryMetrics(ch)
	if len(errMap) > 0 {
		err = fmt.Errorf("queryMetrics returned %d errors", len(errMap))
	}

	return err
}

// 查询监控指标. 先判断是否读取缓存. 禁用缓存或者缓存超时,则读取数据库
func (s *Server) queryMetrics(ch chan<- prometheus.Metric) map[string]error {
	metricErrors := make(map[string]error)
	wg := sync.WaitGroup{}
	limit := newRateLimit(s.parallel)
	for metric, queryInstance := range s.queryInstanceMap {
		wg.Add(1)
		queryInst := queryInstance
		metricName := metric
		limit.getToken()
		go func() {
			defer wg.Done()
			defer limit.putToken()
			err := s.queryMetric(ch, queryInst)
			if err != nil {
				metricErrors[metricName] = err
			}
		}()

	}
	wg.Wait()

	return metricErrors
}

func (s *Server) queryMetric(ch chan<- prometheus.Metric, queryInstance *QueryInstance) error {
	var (
		metric         = queryInstance.Name
		scrapeMetric   = false // Whether to collect indicators from the database 是否从数据库里采集指标
		cachedMetric   = &cachedMetrics{}
		metrics        []prometheus.Metric
		nonFatalErrors []error
		err            error
	)

	// log.Debugf("Querying metric : %s", metric)

	querySQL := queryInstance.GetQuerySQL(s.lastMapVersion)
	if querySQL == nil {
		log.Errorf("Collect Metric %s not define querySQL for version %s", metric, s.lastMapVersion.String())
		return nil
	}
	if strings.EqualFold(querySQL.Status, statusDisable) {
		log.Debugf("Collect Metric %s disable. skip", metric)
		return nil
	}
	// Determine whether to enable caching and cache expiration 判断是否启用缓存和缓存过期
	if !s.disableCache {
		var found bool
		// Check if the metric is cached
		s.cacheMtx.Lock()
		cachedMetric, found = s.metricCache[metric]
		s.cacheMtx.Unlock()
		// If found, check if needs refresh from cache
		if !found {
			scrapeMetric = true
		} else if !cachedMetric.IsValid(queryInstance.TTL) {
			scrapeMetric = true
		}
	} else {
		scrapeMetric = true
	}
	if scrapeMetric {
		metrics, nonFatalErrors, err = s.doCollectMetric(queryInstance)
	} else {
		metrics, nonFatalErrors = cachedMetric.metrics, cachedMetric.nonFatalErrors
	}

	// Serious error - a namespace disappeared
	if err != nil {
		nonFatalErrors = append(nonFatalErrors, err)
		log.Errorf("Collect Metric %s err %s", metric, err)
	}
	// Non-serious errors - likely version or parsing problems.
	if len(nonFatalErrors) > 0 {
		var errText string
		for _, err := range nonFatalErrors {
			log.Errorf("Collect Metric %s nonFatalErrors err %s", metric, err)
			errText += err.Error()
		}
		err = errors.New(errText)
	}

	// Emit the metrics into the channel
	for _, metric := range metrics {
		ch <- metric
	}

	if scrapeMetric && queryInstance.TTL > 0 {
		// Only cache if metric is meaningfully cacheable
		s.cacheMtx.Lock()
		s.metricCache[metric] = &cachedMetrics{
			metrics:        metrics,
			lastScrape:     time.Now(), // 改为查询完时间
			nonFatalErrors: nonFatalErrors,
		}
		s.cacheMtx.Unlock()
	}
	return err
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

	return result, nil
}

func NewServer(dsn string, opts ...ServerOpt) (*Server, error) {
	// 获取server名称 ip:port
	fingerprint, err := parseFingerprint(dsn)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	log.Infof("Established new database connection to %q.", fingerprint)

	s := &Server{
		db:     db,
		dsn:    dsn,
		master: false,
		labels: prometheus.Labels{
			serverLabelName: fingerprint,
		},
		metricCache: make(map[string]*cachedMetrics),
	}

	for _, opt := range opts {
		opt(s)
	}

	db.SetMaxOpenConns(s.parallel)
	// db.SetMaxIdleConns(1)
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
				time.Sleep(time.Duration(errCount) * time.Second)
				continue
			}
			s.servers[dsn] = server
		}
		if err = server.Ping(); err != nil {
			delete(s.servers, dsn)
			time.Sleep(time.Duration(errCount) * time.Second)
			continue
		}
		break
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
