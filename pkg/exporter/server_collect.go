// Copyright © 2021 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"strings"
	"time"
)

// func (s *Server) getCacheMetrics(name string) *cachedMetrics {
// 	s.cacheMtx.Lock()
// 	cachedMetric, found := s.metricCache[name]
// 	s.cacheMtx.Unlock()
// 	if found {
// 		return cachedMetric
// 	}
// 	return &cachedMetrics{}
// }
//
// func (s *Server) collectMetrics(prometheusCh chan<- prometheus.Metric) map[string]error {
// 	var (
// 		metricCh     = make(chan *cachedMetrics, 100)
// 		metricErrors = make(map[string]error)
// 		wg           = sync.WaitGroup{}
// 		limit        = newRateLimit(s.parallel)
// 		doneCh       = make(chan struct{})
// 	)
// 	go func() {
// 		select {
// 		case cacheMetric := <-metricCh:
// 			if cacheMetric.err != nil {
// 				metricErrors[cacheMetric.name] = cacheMetric.err
// 			}
// 			for _, m := range cacheMetric.metrics {
// 				prometheusCh <- m
// 			}
// 		case <-doneCh:
// 			return
// 		}
// 	}()
// 	for name, queryInstance := range s.queryInstanceMap {
// 		cachedMetric := s.getCacheMetrics(name)
// 		if cachedMetric.IsCollect() {
// 			log.Debugf("Collect metric [%s] running. auto skip", name)
// 			continue
// 		}
// 		wg.Add(1)
// 		queryInst := queryInstance
// 		limit.getToken()
// 		go func() {
// 			defer wg.Done()
// 			defer limit.putToken()
// 			s.collectMetric(metricCh, queryInst)
// 		}()
// 	}
// 	wg.Wait()
// 	close(doneCh)
// 	close(metricCh)
// 	return metricErrors
// }
//
// func (s *Server) collectMetric(ch chan<- *cachedMetrics, queryInstance *QueryInstance) *cachedMetrics {
// 	var (
// 		metricName     = queryInstance.Name
// 		scrapeMetric   = false // Whether to collect indicators from the database 是否从数据库里采集指标
// 		cachedMetric   = &cachedMetrics{}
// 		metrics        []prometheus.Metric
// 		nonFatalErrors []error
// 		err            error
// 	)
// 	defer func() {
// 		ch <- cachedMetric
// 	}()
// 	querySQL := queryInstance.GetQuerySQL(s.lastMapVersion)
// 	if querySQL == nil {
// 		log.Errorf("Collect Metric [%s] not define querySQL for version %s", metricName, s.lastMapVersion.String())
// 		return nil
// 	}
// 	if strings.EqualFold(querySQL.Status, statusDisable) {
// 		log.Debugf("Collect metric [%s] disable. auto skip", metricName)
// 		return nil
// 	}
// 	cachedMetric = s.getCacheMetrics(metricName)
// 	// Determine whether to enable caching and cache expiration 判断是否启用缓存和缓存过期
// 	if !s.disableCache || queryInstance.IsEnableCache() {
// 		if !cachedMetric.IsValid(queryInstance.TTL) {
// 			scrapeMetric = true
// 		}
// 	} else {
// 		scrapeMetric = true
// 	}
//
// 	if cachedMetric.err != nil || len(cachedMetric.nonFatalErrors) > 0 {
// 		scrapeMetric = true
// 	}
//
// 	if !scrapeMetric {
// 		log.Infof("Collect metric [%s] use cache", metricName)
// 		return cachedMetric
// 	}
//
// 	cachedMetric.collect = true
// 	metrics, nonFatalErrors, err = s.doCollectMetric(queryInstance)
// 	cachedMetric.collect = false
//
// 	// Non-serious errors - likely version or parsing problems.
// 	if len(nonFatalErrors) > 0 {
// 		var errText string
// 		for _, err := range nonFatalErrors {
// 			log.Errorf("Collect metric [%s] nonFatalErrors err %s", metricName, err)
// 			errText += err.Error()
// 		}
// 		err = errors.New(errText)
// 	}
// 	if scrapeMetric && queryInstance.TTL > 0 {
// 		// Only cache if metricName is meaningfully cacheable
// 		s.cacheMtx.Lock()
// 		s.metricCache[metricName] = &cachedMetrics{
// 			metrics:        metrics,
// 			lastScrape:     time.Now(), // 改为查询完时间
// 			nonFatalErrors: nonFatalErrors,
// 			err:            err,
// 		}
// 		s.cacheMtx.Unlock()
// 	}
// 	return cachedMetric
// }
//
func (s *Server) doCollectMetric(queryInstance *QueryInstance) ([]prometheus.Metric, []error, error) {
	// 根据版本获取查询sql
	query := queryInstance.GetQuerySQL(s.lastMapVersion, s.primary)
	if query == nil {
		// Return success (no pertinent data)
		return []prometheus.Metric{}, []error{}, nil
	}

	// Don't fail on a bad scrape of one metric
	var (
		rows       *sql.Rows
		err        error
		ctx        = context.Background()
		metricName = queryInstance.Name
	)
	begin := time.Now()
	// TODO disable timeout
	if query.Timeout > 0 { // if timeout is provided, use context
		var cancel context.CancelFunc
		log.Debugf("Collect Metric [%s] executing with time limit: %v", query.Name, query.TimeoutDuration())
		ctx, cancel = context.WithTimeout(context.Background(), query.TimeoutDuration())
		defer cancel()
	}
	log.Debugf("Collect Metric [%s] executing sql %s", queryInstance.Name, query.SQL)
	// tx, err := s.db.Begin()
	// if err != nil {
	// 	log.Errorf("Collect Metric [%s] db.Begin err %s", queryInstance.Name, err)
	// 	return nil, nil, err
	// }
	// defer tx.Commit()
	rows, err = s.db.QueryContext(ctx, query.SQL)
	end := time.Now().Sub(begin).Milliseconds()

	log.Debugf("Collect Metric [%s] executing using time %vms", queryInstance.Name, end)
	if err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") {
			log.Errorf("Collect Metric [%s] executing timeout %v", queryInstance.Name, query.TimeoutDuration())
			err = fmt.Errorf("timeout %v %s", query.TimeoutDuration(), err)
		} else {
			log.Errorf("Collect Metric [%s] QueryContext err %s", queryInstance.Name, err)
		}
		return []prometheus.Metric{}, []error{},
			fmt.Errorf("Collect Metric [%s] QueryContext on database %q err %s ", metricName, s, err)
	}
	defer rows.Close()
	var columnNames []string
	columnNames, err = rows.Columns()
	if err != nil {
		log.Errorf("Collect Metric [%s] executing Columns err %s", queryInstance.Name, err)
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
			log.Errorf("Collect Metric [%s] executing rows.Scan err %s", queryInstance.Name, err)
			return []prometheus.Metric{}, []error{}, errors.New(fmt.Sprintln("Error retrieving rows:", metricName, err))
		}

		// Get the label values for this row.
		labels := make([]string, len(queryInstance.LabelNames))
		for idx, label := range queryInstance.LabelNames {
			labels[idx], _ = dbToString(columnData[columnIdx[label]], s.timeToString)
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
				/*
					WITH data AS (SELECT floor(random()*10) AS d FROM generate_series(1,100)),
					         metrics AS (SELECT SUM(d) AS sum, COUNT(*) AS count FROM data),
					         buckets AS (SELECT le, SUM(CASE WHEN d <= le THEN 1 ELSE 0 END) AS d
					                     FROM data, UNNEST(ARRAY[1, 2, 4, 8]) AS le GROUP BY le)
					    SELECT
					      sum AS histogram_sum,
					      count AS histogram_count,
					      ARRAY_AGG(le) AS histogram,
					      ARRAY_AGG(d) AS histogram_bucket,
					      ARRAY_AGG(le) AS missing,
					      ARRAY_AGG(le) AS missing_sum,
					      ARRAY_AGG(d) AS missing_sum_bucket,
					      ARRAY_AGG(le) AS missing_count,
					      ARRAY_AGG(d) AS missing_count_bucket,
					      sum AS missing_count_sum,
					      ARRAY_AGG(le) AS unexpected_sum,
					      ARRAY_AGG(d) AS unexpected_sum_bucket,
					      'data' AS unexpected_sum_sum,
					      ARRAY_AGG(le) AS unexpected_count,
					      ARRAY_AGG(d) AS unexpected_count_bucket,
					      sum AS unexpected_count_sum,
					      'nan'::varchar AS unexpected_count_count,
					      ARRAY_AGG(le) AS unexpected_bytes,
					      ARRAY_AGG(d) AS unexpected_bytes_bucket,
					      sum AS unexpected_bytes_sum,
					      'nan'::bytea AS unexpected_bytes_count
					    FROM metrics, buckets GROUP BY 1,2
				*/
				if col.Histogram {

				} else if strings.EqualFold(col.Usage, MappedMETRIC) {

				} else {
					value, ok := dbToFloat64(columnData[idx])
					if !ok {
						nonfatalErrors = append(nonfatalErrors, errors.New(fmt.Sprintln("Unexpected error parsing column: ", metricName, columnName, columnData[idx])))
						continue
					}
					// Generate the metric
					metric = prometheus.MustNewConstMetric(col.PrometheusDesc, col.PrometheusType, value, labels...)
				}

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
	// if err = rows.Err(); err != nil {
	// 	log.Debugf("Collect Metric [%s] rows.Err() %s", metricName, err)
	// 	return []prometheus.Metric{}, []error{}, err
	// }
	end = time.Now().Sub(begin).Milliseconds()
	log.Debugf("Collect Metric [%s] executing total time %vms", queryInstance.Name, end)
	return metrics, nonfatalErrors, nil
}
