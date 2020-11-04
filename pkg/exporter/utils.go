// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"errors"
	"fmt"
	"github.com/blang/semver"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"net/url"
	"regexp"
	"strings"
)

// ShadowDSN will hide password part of dsn
func ShadowDSN(dsn string) string {
	pDSN, err := url.Parse(dsn)
	if err != nil {
		return ""
	}
	// Blank user info if not nil
	if pDSN.User != nil {
		pDSN.User = url.UserPassword(pDSN.User.Username(), "******")
	}
	return pDSN.String()
}

func Contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

// // parseDatName extract data name part of a dsn
// func parseDatName(dsn string) string {
// 	u, err := url.Parse(dsn)
// 	if err != nil {
// 		return ""
// 	}
// 	return strings.TrimLeft(u.Path, "/")
// }

// // castString will force interface{} into float64
// func castFloat64(t interface{}) float64 {
// 	switch v := t.(type) {
// 	case int64:
// 		return float64(v)
// 	case float64:
// 		return v
// 	case time.Time:
// 		return float64(v.Unix())
// 	case []byte:
// 		strV := string(v)
// 		result, err := strconv.ParseFloat(strV, 64)
// 		if err != nil {
// 			log.Warnf("fail casting []byte to float64: %v", t)
// 			return math.NaN()
// 		}
// 		return result
// 	case string:
// 		result, err := strconv.ParseFloat(v, 64)
// 		if err != nil {
// 			log.Warnf("fail casting string to float64: %v", t)
// 			return math.NaN()
// 		}
// 		return result
// 	case bool:
// 		if v {
// 			return 1.0
// 		}
// 		return 0.0
// 	case nil:
// 		return math.NaN()
// 	default:
// 		log.Warnf("fail casting unknown to float64: %v", t)
// 		return math.NaN()
// 	}
// }
//
// // castString will force interface{} into string
// func castString(t interface{}) string {
// 	switch v := t.(type) {
// 	case int64:
// 		return fmt.Sprintf("%v", v)
// 	case float64:
// 		return fmt.Sprintf("%v", v)
// 	case time.Time:
// 		return fmt.Sprintf("%v", v.Unix())
// 	case nil:
// 		return ""
// 	case []byte:
// 		// Try and convert to string
// 		return string(v)
// 	case string:
// 		return v
// 	case bool:
// 		if v {
// 			return "true"
// 		}
// 		return "false"
// 	default:
// 		log.Warnf("fail casting unknown to string: %v", t)
// 		return ""
// 	}
// }

// parseConstLabels turn param string into prometheus.Labels
func parseConstLabels(s string) prometheus.Labels {
	labels := make(prometheus.Labels)
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return nil
	}

	parts := strings.Split(s, ",")
	for _, p := range parts {
		keyValue := strings.Split(strings.TrimSpace(p), "=")
		if len(keyValue) != 2 {
			log.Errorf(`malformed labels format %q, should be "key=value"`, p)
			continue
		}
		key := strings.TrimSpace(keyValue[0])
		value := strings.TrimSpace(keyValue[1])
		if key == "" || value == "" {
			continue
		}
		labels[key] = value
	}
	if len(labels) == 0 {
		return nil
	}

	return labels
}

// parseCSV will turn a comma separated string into a []string
func parseCSV(s string) (tags []string) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return nil
	}

	parts := strings.Split(s, ",")
	for _, p := range parts {
		if tag := strings.TrimSpace(p); len(tag) > 0 {
			tags = append(tags, tag)
		}
	}

	if len(tags) == 0 {
		return nil
	}
	return
}

func parseVersionSem(versionString string) (semver.Version, error) {
	version := parseVersion(versionString)
	if version != "" {
		return semver.ParseTolerant(version)
	}
	return semver.Version{},
		errors.New(fmt.Sprintln("Could not find a postgres version in string:", versionString))
}
func parseVersion(versionString string) string {
	var versionRegex = regexp.MustCompile(`^(\(\w+|\w+)\s+((\d+)(\.\d+)?(\.\d+)?)`)
	subMatches := versionRegex.FindStringSubmatch(versionString)
	if len(subMatches) > 2 {
		return subMatches[2]
	}
	return ""
}
