// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"bytes"
	"fmt"
	"github.com/blang/semver"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v2"
	"strings"
	// "html/template"
	"text/template"
	"time"
)

const (
	statusEnable   = "enable"
	statusDisable  = "disable"
	defaultVersion = ">=0.0.0"
)

var queryTemplate, _ = template.New("Query").Parse(`
# ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# ┃ {{ .Name }}
# ┃ {{ .Desc }}
# ┣┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈
# ┃ TTL      ┆ {{ .TTL }}
# ┃ Timeout  ┆ {{ .TimeoutDuration }}
# ┣┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈
{{range .ColumnList}}# ┃ {{.}}
{{end}}# ┣┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈
{{range .MetricList}}# ┃ {{.}}
{{end}}# ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
{{.MarshalYAML}}
`)

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
	Public      bool               `yaml:"public,omitempty"`   // autoDiscover下公用指标,只采集一次
	// Private     bool               `yaml:"ttl,omitempty"`
}

type Query struct {
	Name         string       `yaml:"name,omitempty"`    // actual query name, used as metric prefix
	Desc         string       `yaml:"desc,omitempty"`    // description of this metric query
	SQL          string       `yaml:"sql,omitempty"`     // actual query sql 查询sql
	Version      string       `yaml:"version,omitempty"` // Check supported version 查询支持版本
	versionRange semver.Range `yaml:"-"`                 // semver.Range
	Tags         []string     `yaml:"tags,omitempty"`    // tags are used for execution control
	Timeout      float64      `yaml:"timeout,omitempty"` // query execution timeout in seconds
	TTL          float64      `yaml:"ttl,omitempty"`     // caching ttl in seconds
	Status       string       `yaml:"status,omitempty"`  // enable/disable status. 状态是否开启,针对特定版本.
	EnableCache  string       `yaml:"enableCache,omitempty"`
	DbRole       string       `yaml:"dbRole"` // only primary database collector. default false
}

// TimeoutDuration Get timeout settings
func (q *Query) TimeoutDuration() time.Duration {
	return time.Duration(float64(time.Second) * q.Timeout)
}
func (q *Query) IsPrimary() bool {
	if q.DbRole == "" {
		return true
	}
	return strings.EqualFold(q.DbRole, "primary")
}
func (q *Query) IsStandby() bool {
	if q.DbRole == "" {
		return true
	}
	return strings.EqualFold(q.DbRole, "standby")
}

func (q *Query) IsSQL(ver semver.Version, isPrimary bool) bool {
	if isPrimary {
		if !q.IsPrimary() {
			return false
		}
	} else {
		if !q.IsStandby() {
			return false
		}
	}
	if q.versionRange != nil && q.versionRange(ver) {
		return true
	}

	return false
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
		if query.Version == "" {
			query.Version = defaultVersion
		}
		query.versionRange = semver.MustParseRange(query.Version)
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
func (q *QueryInstance) GetQuerySQL(ver semver.Version, isPrimary bool) *Query {
	for _, query := range q.Queries {
		if query.IsSQL(ver, isPrimary) {
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

func (q *QueryInstance) Explain() string {
	buf := new(bytes.Buffer)
	err := queryTemplate.Execute(buf, q)
	if err != nil {
		panic(err)
	}
	// prettyTablesOptions := html2text.NewPrettyTablesOptions()
	// prettyTablesOptions.AutoMergeCells = false
	// prettyTablesOptions.AutoFormatHeader = false
	// contents, err := html2text.FromString(buf.String(), html2text.Options{
	// 	PrettyTables:        true,
	// 	OmitLinks:           true,
	// 	PrettyTablesOptions: prettyTablesOptions,
	// })
	// if err != nil {
	// 	return ""
	// }
	return buf.String()
}

// MarshalYAML will turn query into YAML format
func (q *QueryInstance) MarshalYAML() string {
	// buf := new(bytes.Buffer)
	v := make(map[string]QueryInstance, 1)
	v[q.Name] = *q
	buf, err := yaml.Marshal(v)
	if err != nil {
		panic(err)
	}
	// fmt.Printf(string(buf))
	return string(buf)
}

// ColumnList return ordered column list
func (q *QueryInstance) ColumnList() (res []*Column) {
	res = make([]*Column, len(q.ColumnNames))
	for i, colName := range q.ColumnNames {
		res[i] = q.Columns[colName]
	}
	return
}

// MetricList returns a list of metric generated by this query
func (q *QueryInstance) MetricList() (res []string) {
	labelSignature := strings.Join(q.LabelList(), ",")
	maxSignatureLength := 0
	res = make([]string, len(q.MetricNames))

	for _, metricName := range q.MetricNames {
		metricColumnName := q.Columns[metricName].Name
		if q.Columns[metricName].Rename != "" {
			metricColumnName = q.Columns[metricName].Rename
		}
		if sigLength := len(q.Name) + len(metricColumnName) + len(labelSignature) + 3; sigLength > maxSignatureLength {
			maxSignatureLength = sigLength
		}
	}
	templateString := fmt.Sprintf("%%-%ds %%-8s %%s", maxSignatureLength+1)
	for i, metricName := range q.MetricNames {
		column := q.Columns[metricName]
		metricColumnName := q.Columns[metricName].Name
		if q.Columns[metricName].Rename != "" {
			metricColumnName = q.Columns[metricName].Rename
		}
		metricSignature := fmt.Sprintf("%s_%s{%s}", q.Name, metricColumnName, labelSignature)
		res[i] = fmt.Sprintf(templateString, metricSignature, column.Usage, column.Desc)
	}

	return
}

// LabelList returns a list of label column names
func (q *QueryInstance) LabelList() []string {
	labelNames := make([]string, len(q.LabelNames))
	for i, labelName := range q.LabelNames {
		labelColumn := q.Columns[labelName]
		if labelColumn.Rename != "" {
			labelNames[i] = labelColumn.Rename
		} else {
			labelNames[i] = labelColumn.Name
		}
	}
	return labelNames
}
