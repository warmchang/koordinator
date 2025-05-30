/*
Copyright 2022 The Koordinator Authors.
Copyright 2019 The Kubernetes Authors.

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

package pod

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"

	e2elog "github.com/koordinator-sh/koordinator/test/e2e/framework/log"
)

const (
	// PodDeleteTimeout is how long to wait for a pod to be deleted.
	PodDeleteTimeout = 5 * time.Minute
)

// DeletePodOrFail deletes the pod of the specified namespace and name. Resilient to the pod
// not existing.
func DeletePodOrFail(c clientset.Interface, ns, name string) {
	ginkgo.By(fmt.Sprintf("Deleting pod %s in namespace %s", name, ns))
	err := c.CoreV1().Pods(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return
	}

	expectNoError(err, "failed to delete pod %s in namespace %s", name, ns)
}

// DeletePodWithWait deletes the passed-in pod and waits for the pod to be terminated. Resilient to the pod
// not existing.
func DeletePodWithWait(c clientset.Interface, pod *v1.Pod) error {
	if pod == nil {
		return nil
	}
	return DeletePodWithWaitByName(c, pod.GetName(), pod.GetNamespace())
}

// DeletePodWithWaitByName deletes the named and namespaced pod and waits for the pod to be terminated. Resilient to the pod
// not existing.
func DeletePodWithWaitByName(c clientset.Interface, podName, podNamespace string) error {
	e2elog.Logf("Deleting pod %q in namespace %q", podName, podNamespace)
	err := c.CoreV1().Pods(podNamespace).Delete(context.TODO(), podName, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil // assume pod was already deleted
		}
		return fmt.Errorf("pod Delete API error: %v", err)
	}
	e2elog.Logf("Wait up to %v for pod %q to be fully deleted", PodDeleteTimeout, podName)
	err = WaitForPodNotFoundInNamespace(c, podName, podNamespace, PodDeleteTimeout)
	if err != nil {
		return fmt.Errorf("pod %q was not deleted: %v", podName, err)
	}
	return nil
}

// DeletePodWithGracePeriod deletes the passed-in pod. Resilient to the pod not existing.
func DeletePodWithGracePeriod(c clientset.Interface, pod *v1.Pod, grace int64) error {
	return DeletePodWithGracePeriodByName(c, pod.GetName(), pod.GetNamespace(), grace)
}

// DeletePodsWithGracePeriod deletes the passed-in pods. Resilient to the pods not existing.
func DeletePodsWithGracePeriod(c clientset.Interface, pods []v1.Pod, grace int64) error {
	for _, pod := range pods {
		if err := DeletePodWithGracePeriod(c, &pod, grace); err != nil {
			return err
		}
	}
	return nil
}

// DeletePodWithGracePeriodByName deletes a pod by name and namespace. Resilient to the pod not existing.
func DeletePodWithGracePeriodByName(c clientset.Interface, podName, podNamespace string, grace int64) error {
	e2elog.Logf("Deleting pod %q in namespace %q", podName, podNamespace)
	err := c.CoreV1().Pods(podNamespace).Delete(context.TODO(), podName, *metav1.NewDeleteOptions(grace))
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil // assume pod was already deleted
		}
		return fmt.Errorf("pod Delete API error: %v", err)
	}
	return nil
}
