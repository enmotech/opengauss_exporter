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
		errors.New(fmt.Sprintln("Could not find a openGauss version in string:", versionString))
}
func parseVersion(versionString string) string {
	versionString = strings.TrimSpace(versionString)
	// var versionRegex = regexp.MustCompile(`^(\(\w+|\w+)\s+((\d+)(\.\d+)?(\.\d+)?)`)
	var versionRegex = regexp.MustCompile(`(?i)(openGauss|MogDB)\s+((\d+)(\.\d+)?(\.\d+))`)
	subMatches := versionRegex.FindStringSubmatch(versionString)
	if len(subMatches) > 3 {
		return subMatches[2]
	}
	return ""
}
