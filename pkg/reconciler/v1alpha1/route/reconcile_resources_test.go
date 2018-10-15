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

package route

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/knative/pkg/logging/testing"
	netv1alpha1 "github.com/knative/serving/pkg/apis/networking/v1alpha1"
	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"github.com/knative/serving/pkg/reconciler/v1alpha1/route/resources"
	"github.com/knative/serving/pkg/reconciler/v1alpha1/route/traffic"
)

func TestReconcileClusterIngress_Insert(t *testing.T) {
	_, _, _, c, _, _, _, _ := newTestReconciler(t)
	r := &v1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-route",
			Namespace: "test-ns",
		},
	}
	ci := newTestClusterIngress(r)
	if _, err := c.reconcileClusterIngress(TestContextWithLogger(t), r, ci); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if created, err := c.getClusterIngressForRoute(r); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if diff := cmp.Diff(ci, created); diff != "" {
		t.Errorf("Unexpected diff (-want +got): %v", diff)
	}
}

func TestReconcileVirtualService_Update(t *testing.T) {
	_, _, _, c, _, _, servingInformer, _ := newTestReconciler(t)
	r := &v1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-route",
			Namespace: "test-ns",
		},
	}

	ci := newTestClusterIngress(r)
	if _, err := c.reconcileClusterIngress(TestContextWithLogger(t), r, ci); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if updated, err := c.getClusterIngressForRoute(r); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		servingInformer.Networking().V1alpha1().ClusterIngresses().Informer().GetIndexer().Add(updated)
	}

	// Modify the ingress object to trigger update.
	ci2 := ci.DeepCopy()
	ci2.Labels["foo"] = "bar"
	if _, err := c.reconcileClusterIngress(TestContextWithLogger(t), r, ci2); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if updated, err := c.getClusterIngressForRoute(r); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		if diff := cmp.Diff(ci2, updated); diff != "" {
			t.Errorf("Unexpected diff (-want +got): %v", diff)
		}
		if diff := cmp.Diff(ci, updated); diff == "" {
			t.Error("Expected difference, but found none")
		}
	}
}

func newTestClusterIngress(r *v1alpha1.Route) *netv1alpha1.ClusterIngress {
	tc := &traffic.TrafficConfig{Targets: map[string][]traffic.RevisionTarget{
		"": {{
			TrafficTarget: v1alpha1.TrafficTarget{
				RevisionName: "revision",
				Percent:      100,
			},
			Active: true,
		}}}}
	return resources.MakeClusterIngress(r, tc)
}
