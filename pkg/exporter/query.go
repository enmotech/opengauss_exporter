// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"fmt"
	"github.com/blang/semver"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v2"
	"strings"
	"time"
)

const (
	statusEnable  = "enable"
	statusDisable = "disable"
)

func CheckStatus(s string) (string, error) {
	s = strings.ToLower(s)
	switch s {
	case statusDisable:
		return statusDisable, nil
	case statusEnable, "":
		return statusEnable, nil
	default:
		return "", fmt.Errorf("no support status %s", s)
	}
}

// Query hold the information of how to fetch metric and parse them
type Query struct {
	Name      string      `yaml:"name,omitempty"`      // actual query name, used as metric prefix
	Desc      string      `yaml:"desc,omitempty"`      // description of this metric query
	QuerySQLS []*QuerySQL `yaml:"querySQLS,omitempty"` // 采集SQL
	Metrics   []*Column   `yaml:"metrics,omitempty"`   // metric definition list
	Status    string      `yaml:"status,omitempty"`    // 状态是否开启
	TTL       float64     `yaml:"ttl,omitempty"`       // caching ttl in seconds
	Priority  int         `yaml:"priority,omitempty"`  // 权重,暂时不用
	Timeout   float64     `yaml:"timeout,omitempty"`   // query execution timeout in seconds
	// metrics parsing auxiliaries
	Path        string             `yaml:"-"` // where am I from ?
	Columns     map[string]*Column `yaml:"-"` // column map
	ColumnNames []string           `yaml:"-"` // column names in origin orders
	LabelNames  []string           `yaml:"-"` // column (name) that used as label, sequences matters
	MetricNames []string           `yaml:"-"` // column (name) that used as metric
}

type QuerySQL struct {
	Name              string       `yaml:"name,omitempty"`    // actual query name, used as metric prefix
	SQL               string       `yaml:"sql,omitempty"`     // 查询sql
	SupportedVersions string       `yaml:"version,omitempty"` // 支持版本
	versionRange      semver.Range `yaml:"-"`                 //
	Tags              []string     `yaml:"tags,omitempty"`    // tags are used for execution control
	Timeout           float64      `yaml:"timeout,omitempty"` // query execution timeout in seconds
	TTL               float64      `yaml:"ttl,omitempty"`     // caching ttl in seconds
	Status            string       `yaml:"status,omitempty"`  // 状态是否开启
}

func (q *Query) ToYaml() string {
	buf, err := yaml.Marshal(q)
	if err != nil {
		return ""
	}
	return string(buf)
}

func (q *Query) Check() error {
	if q.Timeout == 0 {
		q.Timeout = 0.1
	}
	if q.Timeout < 0 {
		q.Timeout = 0
	}
	if status, err := CheckStatus(q.Status); err != nil {
		return err
	} else {
		q.Status = status
	}
	// parse query column info
	columns := make(map[string]*Column, len(q.Metrics))
	for _, querySQL := range q.QuerySQLS {
		if querySQL.Timeout == 0 {
			querySQL.Timeout = q.Timeout
		}
		if querySQL.SupportedVersions != "" {
			querySQL.versionRange = semver.MustParseRange(querySQL.SupportedVersions)
		}
		if status, err := CheckStatus(querySQL.Status); err != nil {
			return err
		} else {
			querySQL.Status = status
		}
		if querySQL.Status == "" {
			querySQL.Status = q.Status
		}
		querySQL.Name = q.Name
	}

	var allColumns, labelColumns, metricColumns []string

	for _, column := range q.Metrics {

		if _, isValid := ColumnUsage[column.Usage]; !isValid {
			return fmt.Errorf("column %s have unsupported usage: %s", column.Name, column.Desc)
		}
		column.Usage = strings.ToUpper(column.Usage)
		switch column.Usage {
		case LABEL:
			labelColumns = append(labelColumns, column.Name)
			column.DisCard = true
		case GAUGE:
			metricColumns = append(metricColumns, column.Name)
		case COUNTER:
			metricColumns = append(metricColumns, column.Name)
		}
		allColumns = append(allColumns, column.Name)
		columns[column.Name] = column
	}
	q.Columns, q.ColumnNames, q.LabelNames, q.MetricNames = columns, allColumns, labelColumns, metricColumns
	return nil
}

func (q *Query) GetQuerySQL(ver semver.Version) *QuerySQL {
	for _, q := range q.QuerySQLS {
		if q.versionRange == nil {
			return q
		}
		if q.versionRange(ver) {
			return q
		}
	}
	return nil
}
func (q *Query) GetColumn(colName string, serverLabels prometheus.Labels) *Column {
	if col, ok := q.Columns[colName]; ok {
		switch col.Usage {
		case LABEL:
			col.DisCard = true
		case GAUGE:
			col.PrometheusType = prometheus.GaugeValue
			col.PrometheusDesc = prometheus.NewDesc(fmt.Sprintf("%s_%s", q.Name, col.Name), col.Desc, q.LabelNames, serverLabels)
		case COUNTER:
			col.PrometheusType = prometheus.CounterValue
			col.PrometheusDesc = prometheus.NewDesc(fmt.Sprintf("%s_%s", q.Name, col.Name), col.Desc, q.LabelNames, serverLabels)
			// case MAPPEDMETRIC:
			// 	col.PrometheusType= prometheus.GaugeValue
			// 	col.PrometheusDesc= prometheus.NewDesc(fmt.Sprintf("%s_%s", q.Name, col.Name), col.Desc, q.LabelNames, serverLabels)
			// case DURATION:
			// 	col.PrometheusType= prometheus.GaugeValue
			// 	col.PrometheusDesc= prometheus.NewDesc(fmt.Sprintf("%s_%s_milliseconds", q.Name, col.Name), col.Desc, q.LabelNames, serverLabels)
		}

		return col
	}
	return nil
}

func (q *Query) TimeoutDuration() time.Duration {
	return time.Duration(float64(time.Second) * q.Timeout)
}
func (q *QuerySQL) TimeoutDuration() time.Duration {
	return time.Duration(float64(time.Second) * q.Timeout)
}
