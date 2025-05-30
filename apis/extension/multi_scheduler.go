/*
Copyright 2022 The Koordinator Authors.

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

package extension

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	// LabelSchedulerName is used to specify the internal scheduler name for a pod, overriding the spec.schedulerName.
	LabelSchedulerName = SchedulingDomainPrefix + "/scheduler-name"
)

func GetSchedulerName(pod *corev1.Pod) string {
	if schedulerName, ok := pod.Labels[LabelSchedulerName]; ok {
		return schedulerName
	}
	return pod.Spec.SchedulerName
}
