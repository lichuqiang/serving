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

package status

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	"github.com/knative/pkg/apis/duck"
	"github.com/knative/serving/pkg/apis/networking/v1alpha1"
	clientset "github.com/knative/serving/pkg/client/clientset/versioned/typed/networking/v1alpha1"
	listers "github.com/knative/serving/pkg/client/listers/networking/v1alpha1"
)

// Result is a collection of probe result that are needed to
// set status.
type Result struct {
	Ready   bool
	Reason  string
	Message string
}

// Manager is the interface to sync ClusterIngress loadbalancer readiness
// status updates to the API server.
type Manager interface {
	// SetLoadBalancerReadiness trigger update an update on LoadBalancerReady
	// condition of the ClusterIngress with given probe result.
	SetLoadBalancerReadiness(ingress *v1alpha1.ClusterIngress, res Result) error

	// IsIngressUpToDate provides a way to validate if generation
	// of given ingress is up-do-date.
	IsIngressUpToDate(ingress *v1alpha1.ClusterIngress) bool
}

type manager struct {
	ingressClient        clientset.ClusterIngressInterface
	clusterIngressLister listers.ClusterIngressLister
}

// NewManager returns with a new status manager instance.
func NewManager(client clientset.ClusterIngressInterface, clusterIngressLister listers.ClusterIngressLister) Manager {
	return &manager{
		ingressClient:        client,
		clusterIngressLister: clusterIngressLister,
	}
}

// SetLoadBalancerReadiness is implementation of the interface from Manager.
func (m *manager) SetLoadBalancerReadiness(ingress *v1alpha1.ClusterIngress, res Result) error {
	// Skip updating if the instance received has been out of date.
	if !m.isIngressUpToDate(ingress) {
		return fmt.Errorf("received ClusterIngress %s (generation %v) is stale", ingress.Name, ingress.Spec.Generation)
	}

	newIngress := ingress.DeepCopy()
	newIngress.Status.MarkLoadBalancerNotReady(res.Reason, res.Message)
	if res.Ready {
		newIngress.Status.MarkLoadBalancerReady()
	}

	patch, err := duck.CreateMergePatch(ingress, newIngress)
	if err != nil {
		return err
	}

	// TODO(lichuqiang): consider to improve this if Strategic Merge Patch
	// on CRDs is supported on k8s side later.
	_, err = m.ingressClient.Patch(ingress.Name, types.MergePatchType, patch)

	return err
}

func (m *manager) IsIngressUpToDate(ingress *v1alpha1.ClusterIngress) bool {
	return m.isIngressUpToDate(ingress)
}

func (m *manager) isIngressUpToDate(ingress *v1alpha1.ClusterIngress) bool {
	old, err := m.clusterIngressLister.Get(ingress.Name)
	if err != nil {
		// The lister will not return error unless the ingress can not be found,
		// we'll treat the scenario as out-of-date too.
		return false
	}

	// Skip updating if the instance received has been out of date.
	if old.Generation != ingress.Spec.Generation {
		return false
	}

	return true
}
