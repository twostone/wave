/*
Copyright 2018 Pusher Ltd. and Wave Contributors

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

package core

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// deleteStuckUnschedulablePods deletes the Pods of the StatefulSet that were
// created while Wave had scheduling disabled.
//
// A Pod's spec.schedulerName is immutable, so restoring the scheduler in the
// pod template leaves those Pods waiting forever for a scheduler that does not
// exist. With OrderedReady the StatefulSet controller waits for them to become
// ready before it rolls out the restored revision, so the StatefulSet stays
// wedged until they are deleted and recreated from the current revision.
func (h *Handler[I]) deleteStuckUnschedulablePods(ctx context.Context, statefulSet *appsv1.StatefulSet) error {
	log := logf.Log.WithName("wave").WithValues("namespace", statefulSet.GetNamespace(), "name", statefulSet.GetName())

	// The cache only holds the namespaces given by --namespaces
	pods := &corev1.PodList{}
	if err := h.List(ctx, pods, client.InNamespace(statefulSet.GetNamespace())); err != nil {
		return fmt.Errorf("error listing pods: %v", err)
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		if !isGatedPodOf(pod, statefulSet) {
			continue
		}

		// The UID precondition prevents a stale cache from deleting a healthy
		// Pod which has replaced the gated one
		uid := pod.GetUID()
		if err := h.Delete(ctx, pod, client.Preconditions{UID: &uid}); err != nil {
			if errors.IsNotFound(err) || errors.IsConflict(err) {
				continue
			}
			return fmt.Errorf("error deleting pod %s/%s: %v", pod.GetNamespace(), pod.GetName(), err)
		}

		log.V(0).Info("Deleted pod which was created while scheduling was disabled", "pod", pod.GetName())
		h.recorder.Eventf(statefulSet, corev1.EventTypeNormal, "GatedPodDeleted", "Deleted Pod %s which was created while scheduling was disabled", pod.GetName())
	}

	return nil
}

// isGatedPodOf returns whether the Pod belongs to the StatefulSet and is still
// held back by the scheduler Wave uses to disable scheduling
func isGatedPodOf(pod *corev1.Pod, statefulSet *appsv1.StatefulSet) bool {
	if pod.Spec.SchedulerName != SchedulingDisabledSchedulerName {
		return false
	}
	// Never touch a Pod that has been placed on a node or is already going away
	if pod.Spec.NodeName != "" || pod.GetDeletionTimestamp() != nil {
		return false
	}
	for _, ref := range pod.GetOwnerReferences() {
		if ref.UID == statefulSet.GetUID() {
			return true
		}
	}
	return false
}
