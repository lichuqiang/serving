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

package resources

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/pkg/kmeta"
	netv1alpha1 "github.com/knative/serving/pkg/apis/networking/v1alpha1"
	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"github.com/knative/serving/pkg/reconciler/v1alpha1/route/resources/names"
)

var loadBalancerNotFoundError = errors.New("failed to fetch loadbalancer domain/IP from ingress status")

// MakeK8sService creates a Service that redirect to the loadbalancer specified
// in ClusterIngress status. It's owned by the provided v1alpha1.Route.
// The purpose of this service is to provide a domain name for Istio routing.
func MakeK8sService(route *v1alpha1.Route, ingress *netv1alpha1.ClusterIngress) (*corev1.Service, error) {
	svcSpec, err := makeServiceSpec(ingress)
	if err != nil {
		return nil, err
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.K8sService(route),
			Namespace: route.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				// This service is owned by the Route.
				*kmeta.NewControllerRef(route),
			},
		},
		Spec: *svcSpec,
	}, nil
}

func makeServiceSpec(ingress *netv1alpha1.ClusterIngress) (*corev1.ServiceSpec, error) {
	ingressStatus := ingress.Status
	if ingressStatus.LoadBalancer == nil || len(ingressStatus.LoadBalancer.Ingress) == 0 {
		return nil, loadBalancerNotFoundError
	}
	// TODO(lichuqiang): what to do when we have multiple ingresses here?
	balancer := ingressStatus.LoadBalancer.Ingress[0]

	switch {
	case len(balancer.Domain) != 0:
		return &corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: balancer.Domain,
		}, nil
	case len(balancer.DomainInternal) != 0:
		return &corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: balancer.DomainInternal,
		}, nil
	case len(balancer.IP) != 0:
		// TODO(lichuqiang): what to do if LoadBalancer IP is provided?
		// We'll also need ports info to make it take effect.
	}

	return nil, loadBalancerNotFoundError
}
