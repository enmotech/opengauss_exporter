// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"database/sql"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/blang/semver"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func Test_parseFingerprint(t *testing.T) {
	type args struct {
		url string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "localhost:55432",
			args: args{
				url: "postgres://userDsn:passwordDsn@localhost:55432/?sslmode=disabled",
			},
			want: "'localhost':'55432'",
		},
		{
			name: "localhost:55432",
			args: args{
				url: "postgres://userDsn:passwordDsn%3D@localhost:55432/?sslmode=disabled",
			},
			want: "'localhost':'55432'",
		},
		{
			name: "127.0.0.1:5432",
			args: args{
				url: "user=xxx password=xxx host=127.0.0.1 port=5432 dbname=postgres sslmode=disable",
			},
			want: "127.0.0.1:5432",
		},
		{
			name: "localhost:1234",
			args: args{
				url: "port=1234",
			},

			want: "localhost:1234",
		},
		{
			name: "example:5432",
			args: args{
				url: "host=example",
			},
			want: "example:5432",
		},
		{
			name: "xyz",
			args: args{
				url: "xyz",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFingerprint(tt.args.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFingerprint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_dbToFloat64(t *testing.T) {
	type args struct {
		t interface{}
	}
	tests := []struct {
		name  string
		args  args
		want  float64
		want1 bool
	}{
		{
			name:  "int64",
			args:  args{t: int64(2)},
			want:  float64(2),
			want1: true,
		},
		{
			name:  "float64",
			args:  args{t: float64(2)},
			want:  float64(2),
			want1: true,
		},
		{
			name:  "time.Time",
			args:  args{t: time.Unix(123456790, 0)},
			want:  float64(123456790),
			want1: true,
		},
		{
			name:  "[]byte",
			args:  args{t: []byte("1234")},
			want:  float64(1234),
			want1: true,
		},
		{
			name:  "string",
			args:  args{t: "232.14"},
			want:  232.14,
			want1: true,
		},
		{
			name:  "bool_true",
			args:  args{t: true},
			want:  1.0,
			want1: true,
		},
		{
			name:  "bool_false",
			args:  args{t: false},
			want:  0.0,
			want1: true,
		},
		// {
		// 	name:"nil",
		// 	args: args{t: nil},
		// 	want: math.NaN(),
		// 	want1: true,
		// },
		// {
		// 	name:"string_NaN",
		// 	args: args{t: "NaN"},
		// 	want: math.NaN(),
		// 	want1: true,
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := dbToFloat64(tt.args.t)
			if got != tt.want {
				t.Errorf("dbToFloat64() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("dbToFloat64() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_dbToString(t *testing.T) {
	type args struct {
		t interface{}
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 bool
	}{
		{
			name:  "int64",
			args:  args{t: int64(1)},
			want:  "1",
			want1: true,
		},
		{
			name:  "float64",
			args:  args{t: float64(1.1)},
			want:  "1.1",
			want1: true,
		},
		{
			name:  "time.Time",
			args:  args{t: time.Unix(123456790, 0)},
			want:  "123456790000",
			want1: true,
		},
		{
			name:  "nil",
			args:  args{t: nil},
			want:  "",
			want1: true,
		},
		{
			name:  "[]byte",
			args:  args{t: []byte("a")},
			want:  "a",
			want1: true,
		},
		{
			name:  "string",
			args:  args{t: "a"},
			want:  "a",
			want1: true,
		},
		{
			name:  "bool_true",
			args:  args{t: true},
			want:  "true",
			want1: true,
		},
		{
			name:  "bool_false",
			args:  args{t: false},
			want:  "false",
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := dbToString(tt.args.t, false)
			if got != tt.want {
				t.Errorf("dbToString() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("dbToString() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_Server(t *testing.T) {
	var (
		db  *sql.DB
		err error
		s   = &Server{
			dsn: "",
			db:  nil,
			labels: prometheus.Labels{
				"server": "localhost:5432",
			},
			primary:                false,
			namespace:              "",
			disableSettingsMetrics: false,
			disableCache:           false,
			lastMapVersion: semver.Version{
				Major: 0,
				Minor: 0,
				Patch: 0,
			},
			queryInstanceMap: defaultMonList,
			lock:             sync.RWMutex{},
			metricCache:      map[string]*cachedMetrics{},
			cacheMtx:         sync.Mutex{},
		}
		mock          sqlmock.Sqlmock
		metricName    = "pg_lock"
		queryInstance = defaultMonList[metricName]
	)

	_ = queryInstance.Check()
	t.Run("ServerOpt", func(t *testing.T) {
		s := &Server{
			labels: map[string]string{},
		}
		ServerWithLabels(prometheus.Labels{
			"server": "localhost:5432",
		})(s)
		assert.Equal(t, prometheus.Labels{
			"server": "localhost:5432",
		}, s.labels)

		ServerWithNamespace("a1")(s)
		assert.Equal(t, "a1", s.namespace)
		ServerWithDisableSettingsMetrics(false)(s)
		assert.Equal(t, false, s.disableSettingsMetrics)
		ServerWithDisableCache(false)(s)
		assert.Equal(t, false, s.disableCache)
		ServerWithTimeToString(false)(s)
		assert.Equal(t, false, s.timeToString)
		ServerWithParallel(2)(s)
		assert.Equal(t, 2, s.parallel)
	})
	t.Run("Close", func(t *testing.T) {
		db, mock, err = sqlmock.New()
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectClose()
		err := s.Close()
		assert.NoError(t, err)
	})
	t.Run("Close_nil", func(t *testing.T) {
		s.db = nil
		err := s.Close()
		assert.NoError(t, err)
	})
	t.Run("Ping", func(t *testing.T) {
		db, mock, err = sqlmock.New(sqlmock.MonitorPingsOption(true))
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectPing()
		err := s.Ping()
		assert.NoError(t, err)
	})
	t.Run("Ping_err", func(t *testing.T) {
		db, mock, err = sqlmock.New(sqlmock.MonitorPingsOption(true))
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectPing().WillReturnError(fmt.Errorf("ping error"))
		err := s.Ping()
		assert.Error(t, err)
	})
	t.Run("QueryDatabases", func(t *testing.T) {
		db, mock, err = sqlmock.New(sqlmock.MonitorPingsOption(true))
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows([]string{"datname"}).FromCSVString(`postgres
omm`))
		r, err := s.QueryDatabases()
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{"postgres", "omm"}, r)
	})

	t.Run("IsPrimary", func(t *testing.T) {
		db, mock, err = sqlmock.New(sqlmock.MonitorPingsOption(true))
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectQuery("SELECT pg_is_in_recovery()").WillReturnRows(
			sqlmock.NewRows([]string{"pg_is_in_recovery"}).AddRow(false))
		r, err := s.IsPrimary()
		assert.NoError(t, err)
		assert.Equal(t, true, r)
	})
	t.Run("getVersion", func(t *testing.T) {
		db, mock, err = sqlmock.New(sqlmock.MonitorPingsOption(true))
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows([]string{"version"}).AddRow("PostgreSQL 9.2.4 (openGauss 2.0.0 build 78689da9) compiled at 2021-03-31 21:04:03 commit 0 last mr   on x86_64-unknown-linux-gnu, compiled by g++ (GCC) 7.3.0, 64-bit"))
		err := s.getVersion()
		assert.NoError(t, err)
		assert.Equal(t, "2.0.0", s.lastMapVersion.String())
	})
	t.Run("doCollectMetric", func(t *testing.T) {
		db, mock, err = sqlmock.New()
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows([]string{"datname", "mode", "count"}).FromCSVString(`postgres,AccessShareLock,4
omm,RowShareLock,0
postgres,ShareRowExclusiveLock,0
postgres,ShareLock,0
omm,ShareUpdateExclusiveLock,0
omm,ShareLock,0
omm,RowExclusiveLock,0
omm,AccessShareLock,0
omm,ShareRowExclusiveLock,0
postgres,RowExclusiveLock,0
omm,ExclusiveLock,0
postgres,ExclusiveLock,0
postgres,ShareUpdateExclusiveLock,0
omm,AccessExclusiveLock,0
postgres,RowShareLock,0
postgres,AccessExclusiveLock,0`))
		metrics, errs, err := s.doCollectMetric(queryInstance)
		assert.NoError(t, err)
		assert.ElementsMatch(t, errs, []error{})
		assert.NotNil(t, metrics)
	})
	t.Run("doCollectMetric_NoTimeOut", func(t *testing.T) {
		db, mock, err = sqlmock.New()
		if err != nil {
			t.Error(err)
		}
		s.db = db
		queryInstance.Queries[0].Timeout = 0
		mock.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows([]string{"datname", "mode", "count"}).FromCSVString(`postgres,AccessShareLock,4
omm,RowShareLock,0
postgres,ShareRowExclusiveLock,0
postgres,ShareLock,0
omm,ShareUpdateExclusiveLock,0
omm,ShareLock,0
omm,RowExclusiveLock,0
omm,AccessShareLock,0
omm,ShareRowExclusiveLock,0
postgres,RowExclusiveLock,0
omm,ExclusiveLock,0
postgres,ExclusiveLock,0
postgres,ShareUpdateExclusiveLock,0
omm,AccessExclusiveLock,0
postgres,RowShareLock,0
postgres,AccessExclusiveLock,0`))
		metrics, errs, err := s.doCollectMetric(queryInstance)
		assert.NoError(t, err)
		assert.ElementsMatch(t, errs, []error{})
		assert.NotNil(t, metrics)
	})
	t.Run("doCollectMetric_query_nil", func(t *testing.T) {
		metrics, errs, err := s.doCollectMetric(&QueryInstance{})
		assert.NoError(t, err)
		assert.ElementsMatch(t, []error{}, errs)
		assert.ElementsMatch(t, []prometheus.Metric{}, metrics)
	})
	t.Run("doCollectMetric_timeout", func(t *testing.T) {
		queryInstance.Queries[0].Timeout = 0.1
		db, mock, err = sqlmock.New()
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectQuery("SELECT").WillDelayFor(1 * time.Second).WillReturnRows(
			sqlmock.NewRows([]string{"datname", "mode", "count"}).FromCSVString(`postgres,AccessShareLock,4
omm,RowShareLock,0
postgres,ShareRowExclusiveLock,0
postgres,ShareLock,0
omm,ShareUpdateExclusiveLock,0
omm,ShareLock,0
omm,RowExclusiveLock,0
omm,AccessShareLock,0
omm,ShareRowExclusiveLock,0
postgres,RowExclusiveLock,0
omm,ExclusiveLock,0
postgres,ExclusiveLock,0
postgres,ShareUpdateExclusiveLock,0
omm,AccessExclusiveLock,0
postgres,RowShareLock,0
postgres,AccessExclusiveLock,0`))
		metrics, errs, err := s.doCollectMetric(queryInstance)
		assert.Error(t, err)
		assert.ElementsMatch(t, []error{}, errs)
		assert.ElementsMatch(t, []prometheus.Metric{}, metrics)
	})
	t.Run("doCollectMetric_query_err", func(t *testing.T) {
		db, mock, err = sqlmock.New()
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("error"))
		metrics, errs, err := s.doCollectMetric(queryInstance)
		assert.Error(t, err)
		assert.ElementsMatch(t, []error{}, errs)
		assert.ElementsMatch(t, []prometheus.Metric{}, metrics)
	})
	t.Run("doCollectMetric_query_context deadline exceeded", func(t *testing.T) {
		db, mock, err = sqlmock.New()
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("context deadline exceeded"))
		metrics, errs, err := s.doCollectMetric(queryInstance)
		assert.Error(t, err)
		assert.ElementsMatch(t, []error{}, errs)
		assert.ElementsMatch(t, []prometheus.Metric{}, metrics)
	})
	t.Run("doCollectMetric_pg_stat_replication", func(t *testing.T) {
		queryInstance = pgStatReplication
		queryInstance.Queries[0].Timeout = 100
		err = queryInstance.Check()
		s.lastMapVersion = semver.Version{
			Major: 1,
			Minor: 1,
			Patch: 0,
		}
		if err != nil {
			t.Error(err)
			return
		}
		db, mock, err = sqlmock.New()
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectQuery("SELECT").WillDelayFor(1 * time.Second).WillReturnRows(
			sqlmock.NewRows([]string{"pid", "usesysid", "usename", "application_name", "client_addr", "client_hostname", "client_port", "backend_start", "state", "sender_sent_location",
				"receiver_write_location", "receiver_flush_location", "receiver_replay_location", "sync_priority", "sync_state", "pg_current_xlog_location", "pg_xlog_location_diff",
			}).FromCSVString(`140215315789568,10,omm,"WalSender to Standby","192.168.122.92","kvm-yl2",55802,"2021-01-06 14:45:59.944279+08","Streaming","0/331980B8","0/331980B8","0/331980B8","0/331980B8",1,Sync,"0/331980B8",0`))
		metrics, errs, err := s.doCollectMetric(queryInstance)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []error{}, errs)
		for _, m := range metrics {
			fmt.Printf("%#v\n", m)
		}
		// assert.ElementsMatch(t, []prometheus.Metric{
		// 	&{
		// 		f
		// 	},
		// }, metrics)
	})
	t.Run("doCollectMetric_col_nil", func(t *testing.T) {
		queryInstance = &QueryInstance{
			Name: "a1",
			Desc: "a1",
			Queries: []*Query{
				{
					Name:    "a1",
					SQL:     "select",
					Version: "",
				},
			},
		}
		queryInstance.Queries[0].Timeout = 100
		err = queryInstance.Check()
		s.lastMapVersion = semver.Version{
			Major: 1,
			Minor: 1,
			Patch: 0,
		}
		if err != nil {
			t.Error(err)
			return
		}
		db, mock, err = sqlmock.New()
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectQuery("select").WillDelayFor(1 * time.Second).WillReturnRows(
			sqlmock.NewRows([]string{"a1"}).AddRow(16384))
		_, errs, err := s.doCollectMetric(queryInstance)
		assert.NoError(t, err)
		assert.Equal(t, []error{}, errs)
	})
	t.Run("doCollectMetric_col_nil_err", func(t *testing.T) {
		queryInstance = &QueryInstance{
			Name: "a1",
			Desc: "a1",
			Queries: []*Query{
				{
					Name:    "a1",
					SQL:     "select",
					Version: "",
				},
			},
		}
		queryInstance.Queries[0].Timeout = 100
		err = queryInstance.Check()
		s.lastMapVersion = semver.Version{
			Major: 1,
			Minor: 1,
			Patch: 0,
		}
		if err != nil {
			t.Error(err)
			return
		}
		db, mock, err = sqlmock.New()
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectQuery("select").WillDelayFor(1 * time.Second).WillReturnRows(
			sqlmock.NewRows([]string{"a1"}).AddRow("a1"))
		_, errs, err := s.doCollectMetric(queryInstance)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(errs))
	})
	t.Run("time", func(t *testing.T) {
		now := time.Now()
		fmt.Println(now.Unix())
		fmt.Println(now.Nanosecond())
		fmt.Println(now)
		fmt.Println(fmt.Sprintf("%v%03d", now.Unix(), 00/1000000))
	})
	// t.Run("test_timeout", func(t *testing.T) {
	// 	fmt.Println(uint32(196659))
	// 	dsn := "host=localhost user=sha256test1 password=Test@123 port=5433 dbname=postgres sslmode=disable"
	// 	db, err := sql.Open("opengauss", dsn)
	// 	if err != nil {
	// 		t.Error(err)
	// 	}
	// 	if err := db.Ping(); err != nil {
	// 		t.Error(err)
	// 	}
	// })

	// t.Run("test_timeout", func(t *testing.T) {
	// 	dsn := "host=localhost user=monitor password=mtkOP@128 port=5433 dbname=postgres sslmode=disable"
	// 	db, err := sql.Open("opengauss", dsn)
	// 	if err != nil {
	// 		t.Error(err)
	// 	}
	// 	for i := 0; i < 200; i++ {
	// 		begin := time.Now()
	// 		rows, err := db.Query(`SELECT pid,client_addr,application_name,state,sync_state,lsn,
	//                lsn - sent_location   as sent_diff,
	//        lsn - write_location  as write_diff,
	//        lsn - flush_location  as flush_diff,
	//        lsn - replay_location as replay_diff,
	//        sent_location,write_location,flush_location,replay_location,
	//        backend_uptime,sync_priority
	//        FROM (
	//        SELECT pid,
	//          client_addr,
	//          application_name,
	//          state,
	//          sync_state,
	//          pg_xlog_location_diff(CASE WHEN pg_is_in_recovery() THEN pg_last_xlog_receive_location() ELSE pg_current_xlog_location() END, '0/0') AS lsn,
	//          pg_xlog_location_diff(sender_sent_location,'0/0')                          AS sent_location,
	//          pg_xlog_location_diff(receiver_write_location,'0/0')                         AS write_location,
	//          pg_xlog_location_diff(receiver_flush_location,'0/0')                         AS flush_location,
	//          pg_xlog_location_diff(receiver_replay_location,'0/0')                        AS replay_location,
	//          pg_xlog_location_diff(receiver_replay_location,pg_current_xlog_location())   AS replay_lag,
	//          extract(EPOCH FROM now() - backend_start) AS backend_uptime,
	//          sync_priority
	//     FROM pg_stat_replication) t`)
	//
	// 		if err != nil {
	// 			t.Error(err)
	// 			return
	// 		}
	// 		for rows.Next() {
	// 			var s1, s2, s3, s4, s5, s6, s7, s8, s9, s10, s11, s12, s13, s14, s15, s16, s17 string
	// 			var s18, s19 *string
	// 			if err := rows.Scan(&s1, &s2, &s3, &s4, &s5, &s6, &s7, &s8, &s9, &s10,
	// 				&s11, &s12, &s13, &s14, &s15, &s16, &s17, &s18, &s19); err != nil {
	// 				t.Error(err)
	// 				return
	// 			}
	// 			// fmt.Println(s1, s2)
	// 		}
	// 		if err = rows.Err(); err != nil {
	// 			t.Error(err)
	// 		}
	// 		rows.Close()
	//
	// 		fmt.Printf("%04d :%04vms\n", i, time.Now().Sub(begin).Milliseconds())
	// 		time.Sleep(1 * time.Second)
	// 	}
	// 	// ctx, _ := context.WithTimeout(context.Background(), 80*time.Millisecond)
	//
	// 	// }
	//
	// })

	// t.Run("collectMetric_cache", func(t *testing.T) {
	// 	var (
	// 		metricCh     = make(chan *cachedMetrics, 100)
	// 		cachedMetric = &cachedMetrics{
	// 			metrics:        nil,
	// 			lastScrape:     time.Time{},
	// 			nonFatalErrors: nil,
	// 			err:            nil,
	// 			name:           "",
	// 			collect:        false,
	// 		}
	// 	)
	// 	queryInstance = pgStatReplication
	// 	queryInstance.Queries[0].Timeout = 100
	// 	err = queryInstance.Check()
	// 	s.lastMapVersion = semver.Version{
	// 		Major: 1,
	// 		Minor: 1,
	// 		Patch: 0,
	// 	}
	// 	s.metricCache = map[string]*cachedMetrics{
	// 		"pg_stat_replication": cachedMetric,
	// 	}
	// 	db, mock, err = sqlmock.New()
	// 	if err != nil {
	// 		t.Error(err)
	// 	}
	// 	s.db = db
	// 	mock.ExpectQuery("SELECT").WillDelayFor(1 * time.Second).WillReturnRows(
	// 		sqlmock.NewRows([]string{"pid", "usesysid", "usename", "application_name", "client_addr", "client_hostname", "client_port", "backend_start", "state", "sender_sent_location",
	// 			"receiver_write_location", "receiver_flush_location", "receiver_replay_location", "sync_priority", "sync_state", "pg_current_xlog_location", "pg_xlog_location_diff",
	// 		}).FromCSVString(`140215315789568,10,omm,"WalSender to Standby","192.168.122.92","kvm-yl2",55802,"2021-01-06 14:45:59.944279+08","Streaming","0/331980B8","0/331980B8","0/331980B8","0/331980B8",1,Sync,"0/331980B8",0`))
	// 	s.disableCache = true
	// 	cachedMetric = s.collectMetric(metricCh, queryInstance)
	//
	// 	s.disableCache = false
	// 	queryInstance.EnableCache = statusDisable
	// 	cachedMetric = s.collectMetric(metricCh, queryInstance)
	//
	// 	s.disableCache = false
	// 	queryInstance.EnableCache = statusEnable
	// 	cachedMetric = s.collectMetric(metricCh, queryInstance)
	// 	s.disableCache = false
	// 	queryInstance.EnableCache = ""
	// 	cachedMetric = s.collectMetric(metricCh, queryInstance)
	// })
	t.Run("queryMetric_Primary", func(t *testing.T) {
		s.primary = false
		q := &QueryInstance{
			Name: "test",
			// Primary: true,
		}
		ch := make(chan prometheus.Metric)
		err := s.queryMetric(ch, q)
		assert.NoError(t, err)
	})
	// t.Run("queryMetrics_primary", func(t *testing.T) {
	// 	s.primary = false
	// 	errs := map[string]error{}
	// 	s.queryInstanceMap = map[string]*QueryInstance{
	// 		"test": &QueryInstance{
	// 			Name: "test",
	// 			// Primary: true,
	// 		},
	// 	}
	//
	// 	ch := make(chan prometheus.Metric)
	// 	errs = s.queryMetrics(ch)
	// 	fmt.Println(errs)
	// })
	t.Run("queryMetric_query_nil", func(t *testing.T) {
		var (
			ch = make(chan prometheus.Metric, 100)
			q  = &QueryInstance{}
		)
		q.Queries = nil
		err := s.queryMetric(ch, q)
		assert.NoError(t, err)
	})
	t.Run("queryMetric_query_disable", func(t *testing.T) {
		var (
			ch = make(chan prometheus.Metric, 100)
			q  = pgDatabase
		)
		_ = q.Check()
		q.Queries[0].Status = statusDisable
		err := s.queryMetric(ch, q)
		assert.NoError(t, err)
	})
	t.Run("queryMetric_query_no_cache", func(t *testing.T) {
		var (
			ch = make(chan prometheus.Metric, 100)
			q  = &QueryInstance{
				Name: "pg_database",
				Desc: "OpenGauss Database size",
				Queries: []*Query{
					{
						SQL:     `SELECT datname,size_bytes from dual`,
						Version: ">=0.0.0",
					},
				},
				Metrics: []*Column{
					{Name: "datname", Usage: LABEL, Desc: "Name of this database"},
					{Name: "size_bytes", Usage: GAUGE, Desc: "Disk space used by the database"},
				},
			}
		)
		db, mock, err = sqlmock.New()
		if err != nil {
			t.Error(err)
		}
		s.db = db
		mock.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows([]string{"datname", "size_bytes"}).AddRow("postgres", 1))
		_ = q.Check()
		s.disableCache = true
		err := s.queryMetric(ch, q)
		assert.NoError(t, err)
	})
	t.Run("queryMetric_query_cache", func(t *testing.T) {
		var (
			ch = make(chan prometheus.Metric, 100)
			q  = &QueryInstance{
				Name: "pg_database",
				Desc: "OpenGauss Database size",
				Queries: []*Query{
					{
						SQL:     `SELECT datname,size_bytes from dual`,
						Version: ">=0.0.0",
						TTL:     10,
					},
				},
				Metrics: []*Column{
					{Name: "datname", Usage: LABEL, Desc: "Name of this database"},
					{Name: "size_bytes", Usage: GAUGE, Desc: "Disk space used by the database"},
				},
			}
		)
		db, mock, err = sqlmock.New()
		if err != nil {
			t.Error(err)
		}
		s.db = db
		s.disableCache = false
		desc := prometheus.NewDesc("datname", fmt.Sprintf("Unknown metric from %s", metricName),
			queryInstance.LabelNames, s.labels)
		s.metricCache = map[string]*cachedMetrics{
			"pg_database": {
				metrics: []prometheus.Metric{
					prometheus.MustNewConstMetric(desc,
						prometheus.UntypedValue, 1),
				},
				lastScrape: time.Now().Add(-8 * time.Second),
			},
		}
		err := s.queryMetric(ch, q)

		assert.NoError(t, err)

		// cache 过期
		time.Sleep(3 * time.Second)

		mock.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows([]string{"datname", "size_bytes"}).AddRow("postgres", 1))
		_ = q.Check()
		s.disableCache = true
		err = s.queryMetric(ch, q)
		assert.NoError(t, err)
	})
	t.Run("queryMetric_standby", func(t *testing.T) {
		var (
			ch = make(chan prometheus.Metric, 100)
			q  = &QueryInstance{
				Name: "pg_database",
				Desc: "OpenGauss Database size",
				Queries: []*Query{
					{
						SQL:     `SELECT datname,size_bytes from dual`,
						Version: ">=0.0.0",
						TTL:     10,
						DbRole:  "primary",
					},
				},
				Metrics: []*Column{
					{Name: "datname", Usage: LABEL, Desc: "Name of this database"},
					{Name: "size_bytes", Usage: GAUGE, Desc: "Disk space used by the database"},
				},
			}
		)
		err := s.queryMetric(ch, q)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ch))
	})
	t.Run("queryMetrics", func(t *testing.T) {
		var (
			ch          = make(chan prometheus.Metric, 100)
			pg_database = &QueryInstance{
				Name: "pg_database",
				Desc: "OpenGauss Database size",
				Queries: []*Query{
					{
						SQL:     `SELECT datname,size_bytes from dual`,
						Version: ">=0.0.0",
						TTL:     10,
					},
				},
				Metrics: []*Column{
					{Name: "datname", Usage: LABEL, Desc: "Name of this database"},
					{Name: "size_bytes", Usage: GAUGE, Desc: "Disk space used by the database"},
				},
			}
		)
		_ = pg_database.Check()
		s = &Server{
			parallel: 2,
			queryInstanceMap: map[string]*QueryInstance{
				"pg_database": pg_database,
			},
			metricCache: map[string]*cachedMetrics{},
		}
		db, mock, err = sqlmock.New()
		if err != nil {
			t.Error(err)
		}
		s.db = db

		mock.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows([]string{"datname", "size_bytes"}).AddRow("postgres", 1))
		errs := s.queryMetrics(ch)
		assert.Equal(t, 0, len(errs))
	})
}

func Test_cachedMetrics(t *testing.T) {
	var (
		c = &cachedMetrics{
			metrics:        nil,
			lastScrape:     time.Time{},
			nonFatalErrors: nil,
			err:            nil,
			name:           "",
			collect:        false,
		}
	)
	t.Run("cachedMetrics_IsCollect", func(t *testing.T) {
		assert.Equal(t, c.collect, c.IsCollect())
	})
	t.Run("cachedMetrics_IsValid", func(t *testing.T) {
		// lastScrape := time.Date(2021,04,8,20,25,10,0,time.UTC)
		c := &cachedMetrics{
			metrics:        nil,
			lastScrape:     time.Now(),
			nonFatalErrors: nil,
		}
		// found := true
		assert.Equal(t, c.IsValid(10), true)
		time.Sleep(10 * time.Second)
		assert.Equal(t, c.IsValid(10), false)
	})
}
