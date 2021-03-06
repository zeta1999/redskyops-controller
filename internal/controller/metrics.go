/*
Copyright 2019 GramLabs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"path"
	"runtime"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// ReconcileConflictErrors is a prometheus counter metrics which holds the total
	// number of conflict errors from the Reconciler
	ReconcileConflictErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "controller_runtime_reconcile_conflict_errors_total",
		Help: "Total number of reconciliation conflict errors per controller",
	}, []string{"controller"})
	ReconcileCacheMisses = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "controller_runtime_read_back_misses_total",
		Help: "Total number of times reading back a create trial misses",
	})

	// TODO Experiment is an unbounded label, that might be problematic

	ExperimentTrials = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "redsky_experiment_trials_total",
		Help: "Total number of trials present for an experiment",
	}, []string{"experiment"})
	ExperimentActiveTrials = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "redsky_experiment_active_trials_total",
		Help: "Total number of active trials present for an experiment",
	}, []string{"experiment"})
)

func init() {
	metrics.Registry.MustRegister(
		ReconcileConflictErrors,
		ReconcileCacheMisses,
		ExperimentTrials,
		ExperimentActiveTrials,
	)
}

// guessController dumps stack to try and guess what the controller name should be
func guessController() string {
	pc := make([]uintptr, 3)
	n := runtime.Callers(3, pc)
	if n > 0 {
		frames := runtime.CallersFrames(pc[:n])
		for {
			frame, more := frames.Next()
			if path.Base(path.Dir(frame.File)) == "controllers" {
				p := strings.SplitN(path.Base(frame.File), "_", 2)
				return p[0]
			}

			if !more {
				break
			}
		}
	}
	return "controller"
}
