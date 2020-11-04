// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"fmt"
	"github.com/prometheus/common/log"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

func LoadConfig(configPath string) (queries map[string]*QueryInstance, err error) {
	stat, err := os.Stat(configPath)
	if err != nil {
		return nil, fmt.Errorf("invalid config path: %s: %w", configPath, err)
	}
	if stat.IsDir() { // recursively iterate conf files if a dir is given
		files, err := ioutil.ReadDir(configPath)
		if err != nil {
			return nil, fmt.Errorf("fail reading config dir: %s: %w", configPath, err)
		}

		log.Debugf("load config from dir: %s", configPath)
		confFiles := make([]string, 0)
		for _, conf := range files {
			if !strings.HasSuffix(conf.Name(), ".yaml") && !conf.IsDir() { // depth = 1
				continue // skip non yaml files
			}
			confFiles = append(confFiles, path.Join(configPath, conf.Name()))
		}

		// make global config map and assign priority according to config file alphabetic orders
		// priority is an integer range from 1 to 999, where 1 - 99 is reserved for user
		queries = make(map[string]*QueryInstance)
		var queryCount, configCount int
		for _, confPath := range confFiles {
			if singleQueries, err := LoadConfig(confPath); err != nil {
				log.Warnf("skip config %s due to error: %s", confPath, err.Error())
			} else {
				configCount++
				for name, query := range singleQueries {
					queryCount++
					if query.Priority == 0 { // set to config rank if not manually set
						query.Priority = 100 + configCount
					}
					queries[name] = query // so the later one will overwrite former one
				}
			}
		}
		log.Debugf("load %d of %d queries from %d config files", len(queries), queryCount, configCount)
		return queries, nil
	}

	// single file case: recursive exit condition
	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("fail reading config file %s: %w", configPath, err)
	}
	queries, err = ParseConfig(content, stat.Name())
	if err != nil {
		return nil, err
	}
	log.Debugf("load %d queries from %s, ", len(queries), configPath)
	return queries, nil

}

// ParseConfig turn config content into QueryInstance struct
func ParseConfig(content []byte, path string) (queries map[string]*QueryInstance, err error) {
	queries = make(map[string]*QueryInstance)
	if err = yaml.Unmarshal(content, &queries); err != nil {
		return nil, fmt.Errorf("malformed config: %w", err)
	}

	// parse additional fields
	for name, query := range queries {
		query.Path = path
		if query.Name == "" {
			query.Name = name
		}
		if err := query.Check(); err != nil {
			return nil, err
		}

	}
	return
}
