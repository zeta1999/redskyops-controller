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

package trial

import (
	"fmt"
	"sort"
	"strings"

	redskyv1alpha1 "github.com/redskyops/redskyops-controller/pkg/apis/redsky/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO Make the constant names better reflect the code, not the text
// TODO Use a prefix, like "phase"?
const (
	created      string = "Created"
	setupCreated        = "Setup Created"
	settingUp           = "Setting up"
	setupDeleted        = "Setup Deleted"
	tearingDown         = "Tearing Down"
	patched             = "Patched"
	patching            = "Patching"
	running             = "Running"
	stabilized          = "Stabilized"
	waiting             = "Waiting"
	captured            = "Captured"
	capturing           = "Capturing"
	completed           = "Completed"
	failed              = "Failed"
)

var (
	trialConditionTypeOrder = []redskyv1alpha1.TrialConditionType{
		redskyv1alpha1.TrialSetupCreated,
		redskyv1alpha1.TrialSetupDeleted,
		redskyv1alpha1.TrialPatched,
		redskyv1alpha1.TrialStable,
		redskyv1alpha1.TrialObserved,
		redskyv1alpha1.TrialComplete,
		redskyv1alpha1.TrialFailed,
	}
)

// UpdateStatus will make sure the trial status matches the current state of the trial; returns true only if changes were necessary
func UpdateStatus(t *redskyv1alpha1.Trial) bool {
	phase := summarize(t)
	assignments := assignments(t)
	values := values(t)

	var dirty bool
	if t.Status.Phase != phase {
		t.Status.Phase = phase
		dirty = true
	}
	if t.Status.Assignments != assignments {
		t.Status.Assignments = assignments
		dirty = true
	}
	if t.Status.Values != values {
		t.Status.Values = values
		dirty = true
	}
	return dirty
}

func summarize(t *redskyv1alpha1.Trial) string {
	// If there is an initializer we are in the "setting up" phase
	if t.HasInitializer() {
		return settingUp
	}

	// TODO Re-implement this so it doesn't use conditions, otherwise the conditions need to be ordered
	sort.Slice(t.Status.Conditions, func(i, j int) bool {
		for ii := range trialConditionTypeOrder {
			if trialConditionTypeOrder[ii] == t.Status.Conditions[i].Type {
				for ij := range trialConditionTypeOrder {
					if trialConditionTypeOrder[ij] == t.Status.Conditions[j].Type {
						return ii < ij
					}
				}
			}
		}
		return false
	})

	phase := created
	for i := range t.Status.Conditions {
		c := t.Status.Conditions[i]
		switch c.Type {

		case redskyv1alpha1.TrialSetupCreated:
			switch c.Status {
			case corev1.ConditionTrue:
				phase = setupCreated
			case corev1.ConditionFalse:
				phase = settingUp
			case corev1.ConditionUnknown:
				phase = settingUp
			}

		case redskyv1alpha1.TrialSetupDeleted:
			switch c.Status {
			case corev1.ConditionTrue:
				phase = setupDeleted
			case corev1.ConditionFalse:
				phase = tearingDown
			}

		case redskyv1alpha1.TrialPatched:
			switch c.Status {
			case corev1.ConditionTrue:
				phase = patched
			case corev1.ConditionFalse:
				phase = patching
			case corev1.ConditionUnknown:
				phase = patching
			}

		case redskyv1alpha1.TrialStable:
			switch c.Status {
			case corev1.ConditionTrue:
				if t.Status.StartTime != nil {
					phase = running
				} else {
					phase = stabilized
				}
			case corev1.ConditionFalse:
				phase = waiting
			case corev1.ConditionUnknown:
				phase = waiting
			}

		case redskyv1alpha1.TrialObserved:
			switch c.Status {
			case corev1.ConditionTrue:
				phase = captured
			case corev1.ConditionFalse:
				phase = capturing
			case corev1.ConditionUnknown:
				phase = capturing
			}

		case redskyv1alpha1.TrialComplete:
			switch c.Status {
			case corev1.ConditionTrue:
				return completed
			}

		case redskyv1alpha1.TrialFailed:
			switch c.Status {
			case corev1.ConditionTrue:
				return failed
			}
		}
	}
	return phase
}

func assignments(t *redskyv1alpha1.Trial) string {
	assignments := make([]string, len(t.Spec.Assignments))
	for i := range t.Spec.Assignments {
		assignments[i] = fmt.Sprintf("%s=%d", t.Spec.Assignments[i].Name, t.Spec.Assignments[i].Value)
	}
	return strings.Join(assignments, ", ")
}

func values(t *redskyv1alpha1.Trial) string {
	for i := range t.Status.Conditions {
		c := &t.Status.Conditions[i]
		if c.Type == redskyv1alpha1.TrialFailed && c.Status == corev1.ConditionTrue {
			return c.Message
		}
	}

	values := make([]string, len(t.Spec.Values))
	for i := range t.Spec.Values {
		if t.Spec.Values[i].AttemptsRemaining == 0 {
			values[i] = fmt.Sprintf("%s=%s", t.Spec.Values[i].Name, t.Spec.Values[i].Value)
		}
	}
	return strings.Join(values, ", ")
}

// ApplyCondition updates a the status of an existing condition or adds it if it does not exist
func ApplyCondition(status *redskyv1alpha1.TrialStatus, conditionType redskyv1alpha1.TrialConditionType, conditionStatus corev1.ConditionStatus, reason, message string, time *metav1.Time) {
	// Make sure we have a time
	if time == nil {
		now := metav1.Now()
		time = &now
	}

	// Update an existing condition
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			if status.Conditions[i].Status != conditionStatus {
				// Status change, record the transition
				status.Conditions[i].Status = conditionStatus
				status.Conditions[i].Reason = reason
				status.Conditions[i].Message = message
				status.Conditions[i].LastTransitionTime = *time
				// TODO Is this supposed to update the probe time also?
			} else {
				// Status hasn't changed, update the probe time and reason/message (if necessary)
				status.Conditions[i].LastProbeTime = *time
				if status.Conditions[i].Reason != reason {
					status.Conditions[i].Reason = reason
					status.Conditions[i].Message = message
				}
			}
			return
		}
	}

	// Condition does not exist
	status.Conditions = append(status.Conditions, redskyv1alpha1.TrialCondition{
		Type:               conditionType,
		Status:             conditionStatus,
		Reason:             reason,
		Message:            message,
		LastProbeTime:      *time,
		LastTransitionTime: *time,
	})
}

// CheckCondition checks to see if a condition has a specific status
func CheckCondition(status *redskyv1alpha1.TrialStatus, conditionType redskyv1alpha1.TrialConditionType, conditionStatus corev1.ConditionStatus) bool {
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return status.Conditions[i].Status == conditionStatus
		}
	}

	// If the condition we are looking for *is* unknown, then we did "find" it
	return conditionStatus == corev1.ConditionUnknown
}
