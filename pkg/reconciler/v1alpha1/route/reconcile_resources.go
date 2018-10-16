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
	"sort"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/knative/pkg/logging"
	netv1alpha1 "github.com/knative/serving/pkg/apis/networking/v1alpha1"
	"github.com/knative/serving/pkg/apis/serving"
	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"github.com/knative/serving/pkg/reconciler/v1alpha1/route/resources"
	resourcenames "github.com/knative/serving/pkg/reconciler/v1alpha1/route/resources/names"
)

func (c *Reconciler) getClusterIngressForRoute(route *v1alpha1.Route) (*netv1alpha1.ClusterIngress, error) {
	selector := labels.Set(map[string]string{
		serving.RouteLabelKey:          route.Name,
		serving.RouteNamespaceLabelKey: route.Namespace,
	}).AsSelector()
	ingresses, err := c.clusterIngressLister.List(selector)
	if err != nil {
		return nil, err
	}
	if len(ingresses) == 0 {
		return nil, apierrs.NewNotFound(v1alpha1.Resource("clusteringress"), resourcenames.ClusterIngress(route))
	}
	// Sort the ingresses by name, and use the first one for consistency.
	sort.Sort(ingressByName(ingresses))
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
				"Failed to create ClusterIngress for route %s/%s: %v", r.Namespace, r.Name, err)
			return nil, err
		}
		c.Recorder.Eventf(r, corev1.EventTypeNormal, "Created",
			"Created ClusterIngress %q", clusterIngress.Name)
		return clusterIngress, nil
	} else if err == nil && !equality.Semantic.DeepEqual(clusterIngress.Spec, desired.Spec) {
		clusterIngress.Spec = desired.Spec
		clusterIngress, err = c.ServingClientSet.NetworkingV1alpha1().ClusterIngresses().Update(clusterIngress)
		if err != nil {
			logger.Error("Failed to update ClusterIngress", zap.Error(err))
			return nil, err
		}
		return clusterIngress, nil
	}

	return clusterIngress, err
}

func (c *Reconciler) reconcilePlaceholderService(ctx context.Context, route *v1alpha1.Route) error {
	logger := logging.FromContext(ctx)
	ns := route.Namespace
	name := resourcenames.K8sService(route)

	ingress, err := c.getClusterIngressForRoute(route)
	if apierrs.IsNotFound(err) {
		// Ingress not exist, skip creating.
		logger.Infof("Ingress for route %s/%s, skip creating placeholder k8s service", ns, name)
		return nil
	}
	if err != nil {
		// Return errors other than not found error.
		return err
	}

	desiredService, err := resources.MakeK8sService(route, ingress)
	if err != nil {
		// Loadbalancer not ready, no need to create.
		logger.Warnf("Failed to construct placeholder k8s service: %v", err)
		return nil
	}

	service, err := c.serviceLister.Services(ns).Get(name)
	if apierrs.IsNotFound(err) {
		// Doesn't exist, create it.
		service, err = c.KubeClientSet.CoreV1().Services(ns).Create(desiredService)
		if err != nil {
			logger.Error("Failed to create service", zap.Error(err))
			c.Recorder.Eventf(route, corev1.EventTypeWarning, "CreationFailed",
				"Failed to create service %q: %v", name, err)
			return err
		}
		logger.Infof("Created service %s", name)
		c.Recorder.Eventf(route, corev1.EventTypeNormal, "Created", "Created service %q", name)
	} else if err != nil {
		return err
	} else {
		// Make sure that the service has the proper specification
		// Preserve the ClusterIP field in the Service's Spec, if it has been set.
		desiredService.Spec.ClusterIP = service.Spec.ClusterIP
		if !equality.Semantic.DeepEqual(service.Spec, desiredService.Spec) {
			service.Spec = desiredService.Spec
			service, err = c.KubeClientSet.CoreV1().Services(ns).Update(service)
			if err != nil {
				return err
			}
		}
	}

	// TODO(mattmoor): This is where we'd look at the state of the Service and
	// reflect any necessary state into the Route.
	return nil
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
