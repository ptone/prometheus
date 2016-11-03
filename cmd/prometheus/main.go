// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// The main package for the Prometheus server executable.
package main

import (
	"flag"
	"fmt"
	_ "net/http/pprof" // Comment this line to disable pprof endpoint.
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"

	"github.com/prometheus/prometheus/config"
)

func main() {
	os.Exit(Main())
}

var (
	configSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "prometheus",
		Name:      "config_last_reload_successful",
		Help:      "Whether the last configuration reload attempt was successful.",
	})
	configSuccessTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "prometheus",
		Name:      "config_last_reload_success_timestamp_seconds",
		Help:      "Timestamp of the last successful configuration reload.",
	})
)

func init() {
	prometheus.MustRegister(version.NewCollector("prometheus"))
}

func visitFlag(f *flag.Flag) {
	fmt.Println("# ", f.Usage)
	// TODO consider replacing hyphens with underscores as well to make names
	// more uniform?
	fmt.Println(strings.Replace(f.Name, ".", "_", -1), ": ", f.DefValue)
	fmt.Println()
}

func visitFlagArgs(f *flag.Flag) {
	fmt.Printf("          - \"-%s={{.Values.%s", f.Name, strings.Replace(f.Name, ".", "_", -1))
	if name, _ := flag.UnquoteUsage(f); name == "string" {
		// Not needed, quoted in yaml already
		// fmt.Printf(" | quote ")
	}
	fmt.Printf("}}\"\n")
}

// Main manages the startup and shutdown lifecycle of the entire Prometheus server.
func Main() int {
	if err := parse(os.Args[1:]); err != nil {
		log.Error(err)
		return 2
	}

	if cfg.printVersion {
		// fmt.Fprintln(os.Stdout, version.Print("prometheus"))
		return 0
	}

	cfg.fs.VisitAll(visitFlag)
	cfg.fs.VisitAll(visitFlagArgs)
	return 0
}

// Reloadable things can change their internal state to match a new config
// and handle failure gracefully.
type Reloadable interface {
	ApplyConfig(*config.Config) error
}

func reloadConfig(filename string, rls ...Reloadable) (err error) {
	log.Infof("Loading configuration file %s", filename)
	defer func() {
		if err == nil {
			configSuccess.Set(1)
			configSuccessTime.Set(float64(time.Now().Unix()))
		} else {
			configSuccess.Set(0)
		}
	}()

	conf, err := config.LoadFile(filename)
	if err != nil {
		return fmt.Errorf("couldn't load configuration (-config.file=%s): %v", filename, err)
	}

	failed := false
	for _, rl := range rls {
		if err := rl.ApplyConfig(conf); err != nil {
			log.Error("Failed to apply configuration: ", err)
			failed = true
		}
	}
	if failed {
		return fmt.Errorf("one or more errors occurred while applying the new configuration (-config.file=%s)", filename)
	}
	return nil
}
