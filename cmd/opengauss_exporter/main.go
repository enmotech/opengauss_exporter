// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package main

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"gopkg.in/alecthomas/kingpin.v2"
	"net/http"
	"opengauss_exporter/pkg/exporter"
	"opengauss_exporter/pkg/version"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	defaultPGURL = "postgresql:///?sslmode=disable"
	ogExporter   *exporter.Exporter
	ReloadLock   sync.Mutex
	args         = &Args{}
)

// Args General generic options
type Args struct {
	Help                   *bool   `short:"h" long:"help" description:"Displays help info"`
	Version                *bool   `short:"v" long:"version" description:"Displays mtk version"`
	DbURL                  *string `short:"d" long:"url" description:"openGauss database target url" env:"OG_EXPORTER_URL"`
	ConfigPath             *string `short:"c" long:"config" description:"path to config dir or file" env:"OG_EXPORTER_CONFIG"`
	ConstLabels            *string `short:"l" long:"label" description:"constant lables:comma separated list of label=value pair" env:"OG_EXPORTER_LABEL"`
	ServerTags             *string `short:"t" long:"tags" description:"tags,comma separated list of server tag" env:"OG_EXPORTER_TAG"`
	DisableCache           *bool   `long:"disable-cache" description:"force not using cache" env:"OG_EXPORTER_DISABLE_CACHE"`
	AutoDiscovery          *bool   `long:"auto-discovery" description:"automatically scrape all database for given server" env:"OG_EXPORTER_AUTO_DISCOVERY"`
	ExcludeDatabase        *string `long:"exclude-database" description:"excluded databases when enabling auto-discovery" default:"template0,template1" env:"OG_EXPORTER_EXCLUDE_DATABASE"`
	ExporterNamespace      *string `long:"namespace" description:"prefix of built-in metrics, (og) by default" env:"OG_EXPORTER_NAMESPACE"`
	FailFast               *bool   `long:"fail-fast" description:"fail fast instead of waiting during start-up" env:"OG_EXPORTER_FAIL_FAST"`
	ListenAddress          *string `long:"listen-address" description:"prometheus web server listen address" default:":8080" env:"OG_EXPORTER_LISTEN_ADDRESS"`
	MetricPath             *string `long:"telemetry-path" description:"URL path under which to expose metrics." default:"/metrics" env:"OG_EXPORTER_TELEMETRY_PATH"`
	DryRun                 *bool   `long:"dry-run" description:"dry run and print raw configs"`
	ExplainOnly            *bool   `long:"explain" description:"explain server planned queries"`
	Parallel               *int    `long:"parallel" description:"Specify the parallelism. \nthe degree of parallelism is now useful query database thread "`
	DisableSettingsMetrics *bool
	TimeToString           *bool
}

// RetrieveTargetURL  priority: cli-args > env  > env file path
func (a *Args) RetrieveTargetURL() []string {
	var dsn string
	if a.DbURL != nil && *a.DbURL != "" {
		log.Infof("retrieve target url %s from command line", exporter.ShadowDSN(*a.DbURL))
		dsn = *a.DbURL
	} else {
		if res := os.Getenv("PG_EXPORTER_URL"); res != "" {
			log.Infof("retrieve target url %s from PG_EXPORTER_URL", exporter.ShadowDSN(res))
			dsn = res
		} else if res := os.Getenv("DATA_SOURCE_NAME"); res != "" {
			log.Infof("retrieve target url %s from DATA_SOURCE_NAME", exporter.ShadowDSN(res))
			dsn = res
		} else {
			log.Warnf("fail retrieving target url, fallback on default url: %s", defaultPGURL)
			dsn = defaultPGURL
		}
		a.DbURL = &dsn

	}
	return strings.Split(dsn, ",")
}

// RetrieveConfig  priority: cli-args > env  > env file path
func (a *Args) RetrieveConfig() {
	// priority: cli-args > env  > default settings (check exist)
	if a.ConfigPath != nil && *a.ConfigPath != "" {
		log.Infof("retrieve config path %s from command line", *a.ConfigPath)
		return
	}

	candidate := []string{"og_exporter.yaml", "og_exporter.yml", "/etc/og_exporter.yaml", "/etc/og_exporter"}
	for _, res := range candidate {
		if _, err := os.Stat(res); err == nil { // default1 exist
			log.Infof("fallback on default config path: %s", res)
			a.ConfigPath = &res
			return
		}
	}
}

func initArgs(args *Args) {
	// 增加版本信息
	kingpin.Version(version.GetLongVersion())

	args.DbURL = kingpin.Flag("url", "openGauss database target url").
		Default("").
		Envar("OG_EXPORTER_URL").
		String()
	args.ConfigPath = kingpin.Flag("config", "path to config dir or file.").
		Default("").
		Envar("OG_EXPORTER_CONFIG").
		String()
	args.ConstLabels = kingpin.Flag("constantLabels", "A list of label=value separated by comma(,).").
		Default("").
		Envar("OG_EXPORTER_CONSTANT_LABELS").
		String()
	// args.ServerTags = kingpin.Flag("tags", "tags,comma separated list of server tag").
	// 	Default("").
	// 	Envar("OG_EXPORTER_TAG").
	// 	String()
	args.DisableCache = kingpin.Flag("disable-cache", "force not using cache").
		Default("false").
		Envar("OG_EXPORTER_DISABLE_CACHE").
		Bool()
	args.AutoDiscovery = kingpin.Flag("auto-discover-databases", "Whether to discover the databases on a server dynamically.").
		Default("false").
		Envar("OG_EXPORTER_AUTO_DISCOVER_DATABASES").
		Bool()
	args.ExcludeDatabase = kingpin.Flag("exclude-databases", "A list of databases to remove when autoDiscoverDatabases is enabled").
		Default("template0,template1").
		Envar("OG_EXPORTER_EXCLUDE_DATABASES").
		String()
	args.ExporterNamespace = kingpin.Flag("namespace", "prefix of built-in metrics, (og) by default").
		Default("pg").
		Envar("OG_EXPORTER_NAMESPACE").
		String()
	// args.FailFast = kingpin.Flag("fail-fast", "fail fast instead of waiting during start-up").
	// 	Default("false").
	// 	Envar("OG_EXPORTER_FAIL_FAST").
	// 	Bool()
	args.ListenAddress = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").
		Default(":9187").
		Envar("OG_EXPORTER_WEB_LISTEN_ADDRESS").
		String()
	args.MetricPath = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").
		Default("/metrics").
		Envar("OG_EXPORTER_WEB_TELEMETRY_PATH").
		String()

	args.TimeToString = kingpin.Flag("time-to-string", "convert database timestamp to date string.").
		Default("false").
		Envar("OG_EXPORTER_TIME_TO_STRING").
		Bool()
	args.DryRun = kingpin.Flag("dry-run", "dry run and print default configs and user config").
		Bool()

	args.DisableSettingsMetrics = kingpin.Flag("disable-settings-metrics",
		"Do not include pg_settings metrics.").
		Default("false").
		Envar("OG_EXPORTER_DISABLE_SETTINGS_METRICS").
		Bool()

	args.ExplainOnly = kingpin.Flag("explain", "explain server planned queries").
		Bool()
	args.Parallel = kingpin.Flag("parallel", "Specify the parallelism. \nthe degree of parallelism is now useful query database thread").
		Default("5").
		Envar("OG_EXPORTER_PARALLEL").
		Int()

	log.AddFlags(kingpin.CommandLine)
}

func newOgExporter(args *Args) (*exporter.Exporter, error) {
	dsn := args.RetrieveTargetURL()
	ex, err := exporter.NewExporter(
		exporter.WithDNS(dsn),
		exporter.WithConfig(*args.ConfigPath),
		exporter.WithConstLabels(*args.ConstLabels),
		exporter.WithCacheDisabled(*args.DisableCache),
		// exporter.WithFailFast(*args.FailFast),
		exporter.WithNamespace(*args.ExporterNamespace),
		exporter.WithAutoDiscovery(*args.AutoDiscovery),
		exporter.WithExcludeDatabases(*args.ExcludeDatabase),
		exporter.WithDisableSettingsMetrics(*args.DisableSettingsMetrics),
		exporter.WithTimeToString(*args.TimeToString),
		exporter.WithParallel(*args.Parallel),
		// exporter.WithTags(*args.ServerTags),
	)
	return ex, err

}

func Reload() error {
	ReloadLock.Lock()
	defer ReloadLock.Unlock()
	log.Debugf("reload request received, launch new exporter instance")

	// create a new exporter
	newExporter, err := newOgExporter(args)
	// if launch new exporter failed, do nothing
	if err != nil {
		log.Errorf("fail to reload exporter: %s", err.Error())
		return err
	}

	log.Debugf("shutdown old exporter instance")
	// if older one exists, close and unregister it
	// if ogExporter != nil {
	// 	// DO NOT MANUALLY CLOSE OLD EXPORTER INSTANCE because the stupid implementation of sql.DB
	// 	// there connection will be automatically released after 1 min
	// 	prometheus.Unregister(ogExporter)
	//
	// }
	// prometheus.MustRegister(newExporter)
	ogExporter = newExporter
	log.Infof("server reloaded")
	return nil
}

func runApp(args *Args) {
	// 命令行参数
	initArgs(args)

	kingpin.Parse()

	var err error
	ogExporter, err = newOgExporter(args)
	if err != nil {
		log.Errorf("fail to reload exporter: %s", err.Error())
		return
	}

	if *args.DryRun {
		queryList, err := ogExporter.PrintMetricsList()
		if err != nil {
			log.Error(err)
		}
		fmt.Println(queryList)
		return
	}
	prometheus.MustRegister(ogExporter)
	defer ogExporter.Close()

	router := http.NewServeMux()
	router.Handle(*args.MetricPath, promhttp.Handler())
	// basic information
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		_, _ = w.Write([]byte(`<html><head><title>PG Exporter</title></head><body><h1>PG Exporter</h1><p><a href='` + *args.MetricPath + `'>Metrics</a></p></body></html>`))
	})
	// version report
	router.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		payload := fmt.Sprintf("version %s", version.GetVersion())
		_, _ = w.Write([]byte(payload))
	})

	// reload interface
	router.HandleFunc("/reload", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		if err := Reload(); err != nil {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(fmt.Sprintf("fail to reload: %s", err.Error())))
		} else {
			_, _ = w.Write([]byte(`server reloaded`))
		}
	})

	log.Infof("og_exporter start, listen on http://%s%s", *args.ListenAddress, *args.MetricPath)

	srv := &http.Server{
		Addr:        *args.ListenAddress,
		Handler:     router,
		ReadTimeout: 5 * time.Second,
	}
	go func() {
		// service connections
		// if err := srv.ListenAndServeTLS("server.crt", "server.key"); err != nil && err != http.ErrServerClosed {
		// 	logrus.Fatalf("listen: %s\n", err)
		// }
		if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	closeChan := make(chan struct{}, 1)
	go func() {
		sigChan := make(chan os.Signal, 2)
		signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL, syscall.SIGHUP) //nolint:staticcheck
		defer signal.Stop(sigChan)
		for {
			sig := <-sigChan
			switch sig {
			case syscall.SIGHUP:
				log.Infof("signal %s received, reloading", sig)
				_ = Reload()
			default:
				log.Infof("signal %s received, forcefully terminating", sig)
				closeChan <- struct{}{}
				return
			}
		}
	}()

	<-closeChan
	log.Info("Shutdown Server ...")
	if err = srv.Shutdown(context.Background()); err != nil {
		log.Errorf("Server Shutdown: %s", err)
	}

}

func main() {
	runApp(args)
}
