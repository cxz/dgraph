/*
 * Copyright 2019-2020 Dgraph Labs, Inc. and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package debuginfo

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
)

var pprofProfileTypes = []string{
	"goroutine",
	"heap",
	"threadcreate",
	"block",
	"mutex",
	"profile",
	"trace",
}

var metricTypes = []string{
	"jemalloc",
	"state",
	"health",
}

func saveProfiles(addr, pathPrefix string, duration time.Duration, profiles []string) {
	u, err := url.Parse(addr)
	if err != nil || (u.Host == "" && u.Scheme != "" && u.Scheme != "file") {
		u, err = url.Parse("http://" + addr)
	}
	if err != nil || u.Host == "" {
		glog.Errorf("error while parsing address %s: %s", addr, err)
		return
	}

	for _, profileType := range profiles {
		source := fmt.Sprintf("%s/debug/pprof/%s?duration=%d", u.String(),
			profileType, int(duration.Seconds()))
		savePath := fmt.Sprintf("%s%s.gz", pathPrefix, profileType)

		if err := saveDebug(source, savePath, duration); err != nil {
			glog.Errorf("error while saving pprof profile from %s: %s", source, err)
			continue
		}

		glog.Infof("saving %s profile in %s", profileType, savePath)
	}
}

func saveMetrics(addr, pathPrefix string, duration time.Duration, metrics []string) {
	u, err := url.Parse(addr)
	if err != nil || (u.Host == "" && u.Scheme != "" && u.Scheme != "file") {
		u, err = url.Parse("http://" + addr)
	}
	if err != nil || u.Host == "" {
		glog.Errorf("error while parsing address %s: %s", addr, err)
		return
	}

	for _, metricType := range metrics {
		source := fmt.Sprintf("%s/%s", u.String(),
			metricType)
		savePath := fmt.Sprintf("%s%s.gz", pathPrefix, metricType)

		if err := saveDebug(source, savePath, duration); err != nil {
			glog.Errorf("error while saving metric from %s: %s", source, err)
			continue
		}

		glog.Infof("saving %s metric in %s", metricType, savePath)
	}
}

// saveDebug writes the debug specified in the argument fetching it from the host
// provided in the configuration
func saveDebug(sourceURL, filePath string, duration time.Duration) error {
	var err error
	var resp io.ReadCloser

	glog.Infof("fetching information over HTTP from %s", sourceURL)
	if duration > 0 {
		glog.Info(fmt.Sprintf("please wait... (%v)", duration))
	}

	timeout := duration + duration/2 + 2*time.Second
	resp, err = fetchURL(sourceURL, timeout)
	if err != nil {
		return err
	}

	defer resp.Close()
	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error while creating dump file: %s", err)
	}
	_, err = io.Copy(out, resp)
	return err
}

// fetchURL fetches a profile from a URL using HTTP.
func fetchURL(source string, timeout time.Duration) (io.ReadCloser, error) {
	client := &http.Client{
		Timeout: timeout,
	}
	resp, err := client.Get(source)
	if err != nil {
		return nil, fmt.Errorf("http fetch: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, statusCodeError(resp)
	}

	return resp.Body, nil
}

func statusCodeError(resp *http.Response) error {
	if resp.Header.Get("X-Go-Pprof") != "" &&
		strings.Contains(resp.Header.Get("Content-Type"), "text/plain") {
		if body, err := ioutil.ReadAll(resp.Body); err == nil {
			return fmt.Errorf("server response: %s - %s", resp.Status, body)
		}
	}
	return fmt.Errorf("server response: %s", resp.Status)
}