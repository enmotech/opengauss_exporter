// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"context"
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

type cachedMetrics struct {
	metrics    []prometheus.Metric
	lastScrape time.Time
}

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

type Server struct {
	dsn                    string
	db                     *sql.DB
	labels                 prometheus.Labels
	master                 bool
	namespace              string // default prometheus namespace from cmd args
	disableSettingsMetrics bool
	disableCache           bool
	// Last version used to calculate metric map. If mismatch on scrape,
	// then maps are recalculated.
	lastMapVersion semver.Version
	// Currently active metric map
	queryInstanceMap map[string]*QueryInstance
	mappingMtx       sync.RWMutex
	// Currently cached metrics
	metricCache map[string]cachedMetrics
	cacheMtx    sync.Mutex
}

// Close disconnects from Postgres.
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

	// Start time of collecting metric  采集指标开始时间
	scrapeStart := time.Now()
	for metric, queryInstance := range s.queryInstanceMap {
		log.Debugf("Querying metric : %s", metric)

		querySQL := queryInstance.GetQuerySQL(s.lastMapVersion)
		if querySQL == nil {
			log.Errorf("Querying Metric:%s not define querySQL for version %s", metric, s.lastMapVersion.String())
			continue
		}

		var (
			scrapeMetric   = false // Whether to collect indicators from the database 是否从数据库里采集指标
			cachedMetric   cachedMetrics
			metrics        []prometheus.Metric
			nonFatalErrors []error
			err            error
		)
		// Determine whether to enable caching and cache expiration 判断是否启用缓存和缓存过期
		if !s.disableCache {
			var found bool
			// Check if the metric is cached
			s.cacheMtx.Lock()
			cachedMetric, found = s.metricCache[metric]
			s.cacheMtx.Unlock()
			// If found, check if needs refresh from cache
			if found {
				if scrapeStart.Sub(cachedMetric.lastScrape).Seconds() > queryInstance.TTL {
					scrapeMetric = true
				}
			} else {
				scrapeMetric = true
			}
		} else {
			scrapeMetric = true
		}
		if scrapeMetric {
			metrics, nonFatalErrors, err = s.queryMetric(metric, queryInstance)
		} else {
			metrics = cachedMetric.metrics
		}

		// Serious error - a namespace disappeared
		if err != nil {
			metricErrors[metric] = err
			log.Errorf("collect metric %s err %s", metric, err)
		}
		// Non-serious errors - likely version or parsing problems.
		if len(nonFatalErrors) > 0 {
			for _, err := range nonFatalErrors {
				log.Errorf("collect metric nonFatalErrors %s err %s", metric, err)
			}
		}

		// Emit the metrics into the channel
		for _, metric := range metrics {
			ch <- metric
		}

		if scrapeMetric {
			// Only cache if metric is meaningfully cacheable
			if queryInstance.TTL > 0 {
				s.cacheMtx.Lock()
				s.metricCache[metric] = cachedMetrics{
					metrics:    metrics,
					lastScrape: scrapeStart,
				}
				s.cacheMtx.Unlock()
			}
		}
	}

	return metricErrors
}

// 连接数据查询监控指标
func (s *Server) queryMetric(metricName string, queryInstance *QueryInstance) ([]prometheus.Metric, []error, error) {
	// 根据版本获取查询sql
	query := queryInstance.GetQuerySQL(s.lastMapVersion)
	if query == nil {
		// Return success (no pertinent data)
		return []prometheus.Metric{}, []error{}, nil
	}

	// Don't fail on a bad scrape of one metric
	var rows *sql.Rows
	var err error
	var ctx context.Context

	if query.Timeout != 0 { // if timeout is provided, use context
		var cancel context.CancelFunc
		log.Debugf("queryMetric [%s] executing begin with time limit: %v", query.Name, query.TimeoutDuration())
		ctx, cancel = context.WithTimeout(context.Background(), query.TimeoutDuration())
		defer cancel()

	} else {
		ctx = context.Background()
		defer ctx.Done()
	}
	log.Debugf("queryMetric [%s] executing begin", queryInstance.Name)

	rows, err = s.db.QueryContext(ctx, query.SQL)
	if err != nil {
		return []prometheus.Metric{}, []error{}, fmt.Errorf("Error running queryMetric on database %q query: %s %v ", s, metricName, err)
	}
	defer rows.Close() // nolint: errcheck

	var columnNames []string
	columnNames, err = rows.Columns()
	if err != nil {
		return []prometheus.Metric{}, []error{}, errors.New(fmt.Sprintln("Error retrieving column list for: ", metricName, err))
	}

	// Make a lookup map for the column indices
	var columnIdx = make(map[string]int, len(columnNames))
	for i, n := range columnNames {
		columnIdx[n] = i
	}

	var columnData = make([]interface{}, len(columnNames))
	var scanArgs = make([]interface{}, len(columnNames))
	for i := range columnData {
		scanArgs[i] = &columnData[i]
	}

	nonfatalErrors := []error{}

	metrics := make([]prometheus.Metric, 0)

	for rows.Next() {
		err = rows.Scan(scanArgs...)
		if err != nil {
			return []prometheus.Metric{}, []error{}, errors.New(fmt.Sprintln("Error retrieving rows:", metricName, err))
		}

		// Get the label values for this row.
		labels := make([]string, len(queryInstance.LabelNames))
		for idx, label := range queryInstance.LabelNames {
			labels[idx], _ = dbToString(columnData[columnIdx[label]])
		}

		// Loop over column names, and match to scan data. Unknown columns
		// will be filled with an untyped metric number *if* they can be
		// converted to float64s. NULLs are allowed and treated as NaN.
		for idx, columnName := range columnNames {
			var metric prometheus.Metric
			col := queryInstance.GetColumn(columnName, s.labels)
			if col != nil {
				if col.DisCard {
					continue
				}
				value, ok := dbToFloat64(columnData[idx])
				if !ok {
					nonfatalErrors = append(nonfatalErrors, errors.New(fmt.Sprintln("Unexpected error parsing column: ", metricName, columnName, columnData[idx])))
					continue
				}
				// Generate the metric
				metric = prometheus.MustNewConstMetric(col.PrometheusDesc, col.PrometheusType, value, labels...)

			} else {
				// Unknown metric. Report as untyped if scan to float64 works, else note an error too.
				metricLabel := fmt.Sprintf("%s_%s", metricName, columnName)
				desc := prometheus.NewDesc(metricLabel, fmt.Sprintf("Unknown metric from %s", metricName), queryInstance.LabelNames, s.labels)

				// Its not an error to fail here, since the values are
				// unexpected anyway.
				value, ok := dbToFloat64(columnData[idx])
				if !ok {
					nonfatalErrors = append(nonfatalErrors, errors.New(fmt.Sprintln("Unparseable column type - discarding: ", metricName, columnName, err)))
					continue
				}
				metric = prometheus.MustNewConstMetric(desc, prometheus.UntypedValue, value, labels...)
			}
			metrics = append(metrics, metric)
		}
	}
	return metrics, nonfatalErrors, nil
}

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
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	log.Infof("Established new database connection to %q.", fingerprint)

	s := &Server{
		db:     db,
		dsn:    dsn,
		master: false,
		labels: prometheus.Labels{
			serverLabelName: fingerprint,
		},
		metricCache: make(map[string]cachedMetrics),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// Servers contains a collection of servers to Postgres.
type Servers struct {
	m       sync.Mutex
	servers map[string]*Server
	opts    []ServerOpt
}

// NewServers creates a collection of servers to Postgres.
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
func dbToString(t interface{}) (string, bool) {
	switch v := t.(type) {
	case int64:
		return fmt.Sprintf("%v", v), true
	case float64:
		return fmt.Sprintf("%v", v), true
	case time.Time:
		return fmt.Sprintf("%v", v.Unix()), true
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
		splitted := strings.SplitN(pair, "=", 2)
		if len(splitted) != 2 {
			return "", fmt.Errorf("malformed dsn %q", dsn)
		}
		kv[splitted[0]] = splitted[1]
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
