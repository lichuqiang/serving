/*
Copyright 2018 The Knative Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package route

import (
	"context"
	"reflect"

	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"

	netv1alpha1 "github.com/knative/serving/pkg/apis/networking/v1alpha1"
	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
)

func (c *Reconciler) getClusterIngressForRoute(route *v1alpha1.Route) (*netv1alpha1.ClusterIngress, error) {
	selector := labels.Set(map[string]string{
		"route":    route.Name,
		"route-ns": route.Namespace}).AsSelector()
	ingresses, err := c.clusterIngressLister.List(selector)
	if err != nil {
		return nil, err
	}
	if len(ingresses) == 0 {
		return nil, apierrs.NewNotFound(v1alpha1.Resource("clusteringress"), route.Name)
	}
	// TODO(nghia): decides what to do when we see multiple here?
	// probably sort by name and use the first one for consistency.
	return ingresses[0], nil
}

func (c *Reconciler) reconcileClusterIngress(
	ctx context.Context, r *v1alpha1.Route, desired *netv1alpha1.ClusterIngress) (*netv1alpha1.ClusterIngress, error) {
	logger := logging.FromContext(ctx)
	clusterIngress, err := c.getClusterIngressForRoute(r)
	if apierrs.IsNotFound(err) {
		clusterIngress, err = c.ServingClientSet.NetworkingV1alpha1().ClusterIngresses().Create(desired)
		if err != nil {
			logger.Error("Failed to create ClusterIngress", zap.Error(err))
			c.Recorder.Eventf(r, corev1.EventTypeWarning, "CreationFailed",
				"Failed to create ClusterIngress for route %s.%s: %v", r.Name, r.Namespace, err)
			return nil, err
		}
		c.Recorder.Eventf(r, corev1.EventTypeNormal, "Created",
			"Created ClusterIngress %q", desired.Name)
		return clusterIngress, nil
	} else if err == nil && !equality.Semantic.DeepEqual(clusterIngress.Spec, desired.Spec) {
		// Even though the ClusterIngress is cluster-scoped, the
		// generated API client still require a valid namespace.  Here
		// we just use the Route's.
		clusterIngress.Spec = desired.Spec
		clusterIngress, err = c.ServingClientSet.NetworkingV1alpha1().ClusterIngresses().Update(clusterIngress)
		if err != nil {
			logger.Error("Failed to update ClusterIngress", zap.Error(err))
			return nil, err
		}
		return clusterIngress, nil
	}
	// TODO(mattmoor): This is where we'd look at the state of the VirtualService and
	// reflect any necessary state into the Route.
	return clusterIngress, err
}

// Update the Status of the route.  Caller is responsible for checking
// for semantic differences before calling.
func (c *Reconciler) updateStatus(ctx context.Context, route *v1alpha1.Route) (*v1alpha1.Route, error) {
	existing, err := c.routeLister.Routes(route.Namespace).Get(route.Name)
	if err != nil {
		return nil, err
	}
	// If there's nothing to update, just return.
	if reflect.DeepEqual(existing.Status, route.Status) {
		return existing, nil
	}
	existing.Status = route.Status
	// TODO: for CRD there's no updatestatus, so use normal update.
	updated, err := c.ServingClientSet.ServingV1alpha1().Routes(route.Namespace).Update(existing)
	if err != nil {
		return nil, err
	}

	c.Recorder.Eventf(route, corev1.EventTypeNormal, "Updated", "Updated status for route %q", route.Name)
	return updated, nil
}
