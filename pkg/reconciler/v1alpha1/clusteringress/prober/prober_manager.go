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
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"

	"github.com/knative/serving/pkg/apis/networking/v1alpha1"
	"github.com/knative/serving/pkg/reconciler/v1alpha1/clusteringress/prober/status"
)

const (
	// ProbeServiceName is the name of the probe target service.
	ProbeServiceName = "probe-service"
	// ProbeServicePort is the port of the probe target service.
	ProbeServicePort = 80
	// probeConcurrency is the max number of workers running
	// at the same time.
	probeConcurrency = 10
	// maxRetries is the number of times an instance will be retried
	// before it is dropped out of the queue.
	maxRetries = 15
)

// Manager manages loadbalancer status probing. It creates a probe "worker" for every ClusterIngress
// that specifies a probe. The worker probes its assigned ingress and reports the results.
// The manager will keep re-enqueue the ingress until it is in health state.
type Manager interface {
	// Add enqueue given ClusterIngress for probe.
	Add(ingress *v1alpha1.ClusterIngress)

	// Start starts a pile service as probe target.
	Start(stopCh <-chan struct{})
}

type manager struct {
	// The statusManager provide a way to update API resource status.
	statusManager status.Manager
	// prober executes the probe actions.
	prober *prober
	// WorkQueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workQueue workqueue.RateLimitingInterface
}

// NewManager returns with a Manager instance.
func NewManager(statusManager status.Manager) Manager {
	return &manager{
		statusManager: statusManager,
		prober:        newProber(),
		workQueue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			"NeworkingProber",
		),
	}
}

func (m *manager) Add(ingress *v1alpha1.ClusterIngress) {
	m.workQueue.AddRateLimited(ingress)
}

func (m *manager) Start(stopCh <-chan struct{}) {
	// Start the probe target service.
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("probe request received"))
	})
	go http.ListenAndServe(":7070", mux)

	defer runtime.HandleCrash()
	defer m.workQueue.ShutDown()

	for i := 0; i < probeConcurrency; i++ {
		go wait.Until(func() {
			for m.processNextWorkItem() {
			}
		}, time.Second, stopCh)
	}

	<-stopCh
}

func (m *manager) processNextWorkItem() bool {
	obj, shutdown := m.workQueue.Get()
	if shutdown {
		return false
	}

	defer m.workQueue.Done(obj)

	// We expect ClusterIngress to come off the workqueue.
	ingress, ok := obj.(*v1alpha1.ClusterIngress)
	if !ok {
		// As the item in the workqueue is actually invalid, we call
		// Forget here else we'd go into a loop of attempting to
		// process a work item that is invalid.
		m.workQueue.Forget(obj)
		return true
	}

	w := newWorker(ingress, m)
	shouldRetry := w.run()
	if shouldRetry && m.workQueue.NumRequeues(obj) < maxRetries {
		// Retry the instance as it needs another probe.
		m.workQueue.AddRateLimited(obj)
		return true
	}

	// Finally, if no error occurs we Forget this item so it does not
	// get queued again until another change happens.
	m.workQueue.Forget(obj)
	return true
}
