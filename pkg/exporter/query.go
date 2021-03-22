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
	statusEnable   = "enable"
	statusDisable  = "disable"
	defaultVersion = ">=0.0.0"
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

// QueryInstance hold the information of how to fetch metric and parse them
type QueryInstance struct {
	Name        string             `yaml:"name,omitempty"`    // actual query name, used as metric prefix
	Desc        string             `yaml:"desc,omitempty"`    // description of this metric query
	Queries     []*Query           `yaml:"query,omitempty"`   // 采集SQL
	Metrics     []*Column          `yaml:"metrics,omitempty"` // metric definition list
	Status      string             `yaml:"status,omitempty"`  // enable/disable status. For the entire collection of indicators 针对整个采集指标
	EnableCache string             `yaml:"enableCache,omitempty"`
	TTL         float64            `yaml:"ttl,omitempty"`      // caching ttl in seconds
	Priority    int                `yaml:"priority,omitempty"` // 权重,暂时不用
	Timeout     float64            `yaml:"timeout,omitempty"`  // query execution timeout in seconds
	Path        string             `yaml:"-"`                  // where am I from ?
	Columns     map[string]*Column `yaml:"-"`                  // column map
	ColumnNames []string           `yaml:"-"`                  // column names in origin orders
	LabelNames  []string           `yaml:"-"`                  // column (name) that used as label, sequences matters
	MetricNames []string           `yaml:"-"`                  // column (name) that used as metric
}

type Query struct {
	Name              string       `yaml:"name,omitempty"`    // actual query name, used as metric prefix
	SQL               string       `yaml:"sql,omitempty"`     // actual query sql 查询sql
	SupportedVersions string       `yaml:"version,omitempty"` // Check supported version 查询支持版本
	versionRange      semver.Range `yaml:"-"`                 // semver.Range
	Tags              []string     `yaml:"tags,omitempty"`    // tags are used for execution control
	Timeout           float64      `yaml:"timeout,omitempty"` // query execution timeout in seconds
	TTL               float64      `yaml:"ttl,omitempty"`     // caching ttl in seconds
	Status            string       `yaml:"status,omitempty"`  // enable/disable status. 状态是否开启,针对特定版本.
	EnableCache       string       `yaml:"enableCache,omitempty"`
}

// TimeoutDuration Get timeout settings
func (q *Query) TimeoutDuration() time.Duration {
	return time.Duration(float64(time.Second) * q.Timeout)
}

// TimeoutDuration Get timeout settings
func (q *QueryInstance) TimeoutDuration() time.Duration {
	return time.Duration(float64(time.Second) * q.Timeout)
}

func (q *QueryInstance) ToYaml() string {
	buf, err := yaml.Marshal(q)
	if err != nil {
		return ""
	}
	return string(buf)
}

// Check configuration and handle default values 检查配置并处理默认值
func (q *QueryInstance) Check() error {
	if q.Timeout == 0 {
		q.Timeout = 0.1
	}
	if q.Timeout < 0 {
		q.Timeout = 0
	}
	if q.TTL == 0 {
		q.TTL = 60
	}
	if status, err := CheckStatus(q.Status); err != nil {
		return err
	} else {
		q.Status = status
	}
	// parse query column info
	columns := make(map[string]*Column, len(q.Metrics))
	for _, query := range q.Queries {
		if query.Timeout == 0 {
			query.Timeout = q.Timeout
		}
		if query.EnableCache == "" {
			query.EnableCache = q.EnableCache
		}
		//  默认版本
		if query.SupportedVersions == "" {
			query.SupportedVersions = defaultVersion
		}
		query.versionRange = semver.MustParseRange(query.SupportedVersions)
		if status, err := CheckStatus(query.Status); err != nil {
			return err
		} else {
			query.Status = status
		}
		if query.TTL == 0 {
			query.TTL = q.TTL
		}
		query.Name = q.Name
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
		case DISCARD:
			column.DisCard = true
		case GAUGE:
			metricColumns = append(metricColumns, column.Name)
		case COUNTER:
			metricColumns = append(metricColumns, column.Name)
		case HISTOGRAM:
			column.Histogram = true
			metricColumns = append(metricColumns, column.Name)
		case MappedMETRIC:
			metricColumns = append(metricColumns, column.Name)
		case DURATION:
			metricColumns = append(metricColumns, column.Name)
		}
		allColumns = append(allColumns, column.Name)
		columns[column.Name] = column
	}
	q.Columns, q.ColumnNames, q.LabelNames, q.MetricNames = columns, allColumns, labelColumns, metricColumns
	return nil
}

// GetQuerySQL Get query sql according to version
func (q *QueryInstance) GetQuerySQL(ver semver.Version) *Query {
	for _, query := range q.Queries {
		if query.versionRange == nil {
			return query
		}
		if query.versionRange(ver) {
			return query
		}
	}
	return nil
}
func (q *QueryInstance) IsEnableCache() bool {
	return strings.EqualFold(q.EnableCache, statusEnable)
}

// GetColumn Get column information
func (q *QueryInstance) GetColumn(colName string, serverLabels prometheus.Labels) *Column {
	if col, ok := q.Columns[colName]; ok {
		switch col.Usage {
		case LABEL, DISCARD:
			col.DisCard = true
		case GAUGE:
			col.PrometheusType = prometheus.GaugeValue
			col.PrometheusDesc = prometheus.NewDesc(fmt.Sprintf("%s_%s", q.Name, col.Name), col.Desc, q.LabelNames, serverLabels)
		case COUNTER:
			col.PrometheusType = prometheus.CounterValue
			col.PrometheusDesc = prometheus.NewDesc(fmt.Sprintf("%s_%s", q.Name, col.Name), col.Desc, q.LabelNames, serverLabels)
		case HISTOGRAM:
			col.PrometheusType = prometheus.UntypedValue
			col.PrometheusDesc = prometheus.NewDesc(fmt.Sprintf("%s_%s", q.Name, col.Name), col.Desc, q.LabelNames, serverLabels)
		case MappedMETRIC:
			col.PrometheusType = prometheus.GaugeValue
			col.PrometheusDesc = prometheus.NewDesc(fmt.Sprintf("%s_%s", q.Name, col.Name), col.Desc, q.LabelNames, serverLabels)
		case DURATION:
			col.PrometheusType = prometheus.GaugeValue
			col.PrometheusDesc = prometheus.NewDesc(fmt.Sprintf("%s_%s_milliseconds", q.Name, col.Name), col.Desc, q.LabelNames, serverLabels)
		}

		return col
	}
	return nil
}
