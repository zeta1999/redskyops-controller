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

package check

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"time"

	cmdutil "github.com/redskyops/redskyops-controller/pkg/redskyctl/util"
	redskyapi "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/spf13/cobra"
)

const (
	checkServerLong    = `Check the Red Sky Ops server`
	checkServerExample = ``
)

type CheckServerOptions struct {
	Name           string
	ParameterCount int
	MetricCount    int
	AllowInvalid   bool
	ReportFailure  bool
	DryRun         bool

	RedSkyAPI redskyapi.API

	cmdutil.IOStreams
}

func NewCheckServerOptions(ioStreams cmdutil.IOStreams) *CheckServerOptions {
	return &CheckServerOptions{
		IOStreams: ioStreams,
	}
}

func NewServerCheckCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewCheckServerOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "server",
		Short:   "Check the server",
		Long:    checkServerLong,
		Example: checkServerExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(cmd, o.Complete(f, cmd))
			cmdutil.CheckErr(cmd, o.Validate())
			cmdutil.CheckErr(cmd, o.Run())
		},
	}

	cmd.Flags().IntVar(&o.ParameterCount, "parameters", o.ParameterCount, "Specify the number of experiment parameters to generate (1 - 20).")
	cmd.Flags().IntVar(&o.MetricCount, "metrics", o.MetricCount, "Specify the number of experiment metrics to generate (1 or 2).")
	cmd.Flags().BoolVar(&o.AllowInvalid, "invalid", o.AllowInvalid, "Skip client side validity checks (server enforcement).")
	cmd.Flags().BoolVar(&o.ReportFailure, "fail", o.ReportFailure, "Report an experiment failure instead of generated values.")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", o.DryRun, "Generate experiment JSON to stdout.")

	return cmd
}

func (o *CheckServerOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	// Randomly assign parameter and metric counts if they are not provided
	if o.ParameterCount == 0 {
		o.ParameterCount = rand.Intn(5) + 1
	}
	if o.MetricCount == 0 {
		o.MetricCount = 1
	}

	if !o.DryRun {
		var err error
		o.RedSkyAPI, err = f.RedSkyAPI()
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *CheckServerOptions) Validate() error {
	if !o.AllowInvalid {
		if o.ParameterCount < 1 || o.ParameterCount > 20 {
			return fmt.Errorf("invalid parameter count: %d (should be [1,20])", o.ParameterCount)
		}
		if o.MetricCount < 1 || o.MetricCount > 2 {
			return fmt.Errorf("invalid metric count: %d (should be [1,2]", o.MetricCount)
		}
	}
	return nil
}

func (o *CheckServerOptions) Run() error {
	var err error

	// Generate an experiment
	n := o.Name
	e := generateExperiment(o)

	// If this is a dry run, just write it out
	if o.DryRun {
		return doDryRun(o.Out, n, e)
	}

	// Create the experiment
	var exp redskyapi.Experiment
	if n != "" {
		exp, err = o.RedSkyAPI.CreateExperiment(context.TODO(), redskyapi.NewExperimentName(n), *e)
	} else {
		// If we are generating the name randomly, account for a small number of conflicts
		for i := 0; i < 10; i++ {
			n = GetRandomName(i)
			exp, err = o.RedSkyAPI.CreateExperiment(context.TODO(), redskyapi.NewExperimentName(n), *e)
			if err != nil {
				if aerr, ok := err.(*redskyapi.Error); ok && aerr.Type == redskyapi.ErrExperimentNameConflict {
					continue
				}
			}
			break
		}
	}
	if err != nil {
		return err
	}
	defer func() {
		_ = o.RedSkyAPI.DeleteExperiment(context.TODO(), exp.Self)
	}()

	// Validate the experiment
	if err = checkExperiment(n, e, &exp); err != nil {
		return err
	}

	// Get the next trial assignments
	var t redskyapi.TrialAssignments
	for i := 0; i < 5; i++ {
		t, err = o.RedSkyAPI.NextTrial(context.TODO(), exp.NextTrial)
		if aerr, ok := err.(*redskyapi.Error); ok && aerr.Type == redskyapi.ErrTrialUnavailable {
			time.Sleep(aerr.RetryAfter)
			continue
		}
		break
	}
	if err != nil {
		return err
	}

	// Validate the trial assignments
	if err = checkTrialAssignments(&exp, &t); err != nil {
		return err
	}

	// Report a trial observation back
	v := generateObservation(o, &exp)
	err = o.RedSkyAPI.ReportTrial(context.TODO(), t.ReportTrial, *v)
	if err != nil {
		return err
	}

	// Much success!
	return nil
}

// Serialize the experiment as JSON
// TODO We use JSON instead of YAML here only so we can pipe it to jq, make that configurable?
func doDryRun(out io.Writer, name string, experiment *redskyapi.Experiment) error {
	if name == "" {
		name = GetRandomName(0)
	}
	experiment.DisplayName = name
	b, err := json.MarshalIndent(experiment, "", "    ")
	if err != nil {
		return err
	}
	_, err = out.Write(b)
	return err
}

// Generates an experiment
func generateExperiment(o *CheckServerOptions) *redskyapi.Experiment {
	e := &redskyapi.Experiment{}

	// TODO Optimization?

	used := make(map[string]bool, o.ParameterCount+o.MetricCount)

	for i := 0; i < o.ParameterCount; i++ {
		e.Parameters = append(e.Parameters, redskyapi.Parameter{
			Name:   getUnique(used, GetRandomParameter),
			Type:   redskyapi.ParameterTypeInteger,
			Bounds: *generateBounds(),
		})
	}

	for i := 0; i < o.MetricCount; i++ {
		e.Metrics = append(e.Metrics, redskyapi.Metric{
			Name:     getUnique(used, GetRandomMetric),
			Minimize: generateMinimize(),
		})
	}

	return e
}

func generateObservation(o *CheckServerOptions, exp *redskyapi.Experiment) *redskyapi.TrialValues {
	vals := &redskyapi.TrialValues{}
	if o.ReportFailure {
		vals.Failed = true
	} else {
		for _, m := range exp.Metrics {
			v := redskyapi.Value{MetricName: m.Name}
			v.Value, v.Error = generateValue()
			vals.Values = append(vals.Values, v)
		}
	}
	return vals
}

func generateBounds() *redskyapi.Bounds {
	var min, max int
	for min == max {
		min, max = rand.Intn(100), rand.Intn(4000)
	}
	if min > max {
		min, max = max, min
	}
	return &redskyapi.Bounds{
		Min: json.Number(strconv.Itoa(min)),
		Max: json.Number(strconv.Itoa(max)),
	}
}

func generateMinimize() bool {
	return rand.Intn(2) != 0
}

func generateValue() (float64, float64) {
	// TODO Should we send values greater then 1?
	// TODO Should we send an error?
	return rand.Float64(), 0
}

func checkExperiment(name string, original, created *redskyapi.Experiment) error {
	if created.Self == "" {
		return fmt.Errorf("server did not return a self link")
	}
	if created.NextTrial == "" {
		return fmt.Errorf("server did not return a next trial link")
	}
	if created.Trials == "" {
		return fmt.Errorf("server did not return a trials link")
	}

	// TODO Optimization

	if len(created.Parameters) != len(original.Parameters) {
		return fmt.Errorf("server returned a different number of parameters: %d (expected %d)", len(created.Parameters), len(original.Parameters))
	}
	params := make(map[string]*redskyapi.Parameter, len(original.Parameters))
	for i := range original.Parameters {
		params[original.Parameters[i].Name] = &original.Parameters[i]
	}
	for _, p := range created.Parameters {
		if op, ok := params[p.Name]; ok {
			if p.Bounds.Min != op.Bounds.Min || p.Bounds.Max != op.Bounds.Max {
				return fmt.Errorf("server returned parameter with incorrect bounds: %s [%s,%s] (expected [%s,%s])", p.Name, p.Bounds.Min, p.Bounds.Min, op.Bounds.Min, op.Bounds.Max)
			}
		} else {
			return fmt.Errorf("server returned unexpected parameter: %s", p.Name)
		}
	}

	if len(created.Metrics) != len(original.Metrics) {
		return fmt.Errorf("server returned a different number of metrics: %d (expected %d)", len(created.Metrics), len(original.Metrics))
	}
	metrics := make(map[string]*redskyapi.Metric, len(original.Metrics))
	for i := range original.Metrics {
		metrics[original.Metrics[i].Name] = &original.Metrics[i]
	}
	for _, m := range created.Metrics {
		if om, ok := metrics[m.Name]; ok {
			if m.Minimize != om.Minimize {
				return fmt.Errorf("server returned metric with incorrect minimization: %s [%t]", m.Name, m.Minimize)
			}
		} else {
			return fmt.Errorf("server returned unexpected metric: %s", m.Name)
		}
	}

	return nil
}

func checkTrialAssignments(exp *redskyapi.Experiment, t *redskyapi.TrialAssignments) error {
	if t.ReportTrial == "" {
		return fmt.Errorf("server did not return a report trial link")
	}

	if len(t.Assignments) != len(exp.Parameters) {
		return fmt.Errorf("server returned a different number of parameters: %d (expected %d)", len(t.Assignments), len(exp.Parameters))
	}
	params := make(map[string]*redskyapi.Parameter, len(exp.Parameters))
	for i := range exp.Parameters {
		params[exp.Parameters[i].Name] = &exp.Parameters[i]
	}
	for _, a := range t.Assignments {
		if p, ok := params[a.ParameterName]; ok {
			// Check bounds using floating point arithmetic
			v, err := a.Value.Float64()
			if err != nil {
				return err
			}
			min, err := p.Bounds.Min.Float64()
			if err != nil {
				return err
			}
			max, err := p.Bounds.Max.Float64()
			if err != nil {
				return err
			}
			if v < min || v > max {
				return fmt.Errorf("server return out of bounds assignment: %s = %s (expected [%s,%s])", a.ParameterName, a.Value, p.Bounds.Min, p.Bounds.Max)
			}
		} else {
			return fmt.Errorf("server returned unexpected assignment: %s", a.ParameterName)
		}
	}

	return nil
}
