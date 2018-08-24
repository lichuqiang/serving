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
	"sort"
	"strings"

	"github.com/knative/pkg/kmeta"
	"github.com/knative/serving/pkg/apis/networking/v1alpha1"
	"github.com/knative/serving/pkg/reconciler/v1alpha1/clusteringress/resources/names"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MakeK8sService creates a Service that targets nothing, owned
// by the provided v1alpha1.ClusterIngress.  The purpose of this service is to
// provide a domain name for Istio routing.
func MakeK8sServices(i *v1alpha1.ClusterIngress) []corev1.Service {
	visited := make(map[string]interface{})
	hosts := []string{}

	for _, r := range i.Spec.Rules {
		for _, h := range r.Hosts {
			if strings.HasSuffix(h, ".svc.cluster.local") {
				host := strings.TrimSuffix(h, ".svc.cluster.local")
				if _, existed := visited[host]; !existed {
					hosts = append(hosts, host)
					visited[host] = true
				}
			}
		}
	}
	sort.Strings(hosts)
	services := []corev1.Service{}
	for _, host := range hosts {
		parts := strings.SplitN(host, ".", 2)
		name, ns := parts[0], parts[1]
		services = append(services, corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
				OwnerReferences: []metav1.OwnerReference{
					// This service is owned by the ClusterIngress.
					*kmeta.NewControllerRef(i),
				},
			},
			Spec: corev1.ServiceSpec{
				Type:         corev1.ServiceTypeExternalName,
				ExternalName: names.K8sGatewayServiceFullname,
			},
		})
	}
	return services
}
