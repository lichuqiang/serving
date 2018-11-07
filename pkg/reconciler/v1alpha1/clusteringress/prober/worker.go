/*
Copyright 2018 The Knative Authors

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

package prober

import (
	"github.com/knative/serving/pkg/apis/networking/v1alpha1"
)

// worker handles the probing of its assigned ClusterIngress.
// The worker will block until probe result returned, or the stop channel is closed.
// The worker uses the probe Manager's statusManager for final API update.
// Note: here we won't provide way to explicitly stop an ongoing probe, as it can
// automatically terminate in at most 5 seconds.
type worker struct {
	ingress      *v1alpha1.ClusterIngress
	probeManager *manager
}

// Creates and starts a new probe worker.
func newWorker(ingress *v1alpha1.ClusterIngress, m *manager) *worker {
	w := &worker{
		ingress:      ingress,
		probeManager: m,
	}

	return w
}

// run calls prober for its ClusterIngress status.
func (w *worker) run() (shouldRetry bool) {
	// Fast fail if the ClusterIngress assigned has been out-of-date.
	if !w.probeManager.statusManager.IsIngressUpToDate(w.ingress) {
		return false
	}

	result, err := w.probeManager.prober.probe(w.ingress.Name, w.ingress.Spec.Generation)
	if err != nil {
		// Error during probe, need retry.
		return true
	}
	// upload the probe results.
	if err := w.probeManager.statusManager.SetLoadBalancerReadiness(w.ingress, result); err != nil {
		// Failed to update ingress status, need retry.
		return true
	}

	if result.Ready {
		return false
	}

	// Ingress instance still unhealthy, will retry later.
	return true
}
