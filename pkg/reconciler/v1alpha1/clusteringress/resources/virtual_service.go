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
	"fmt"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	istiov1alpha1 "github.com/knative/pkg/apis/istio/common/v1alpha1"
	"github.com/knative/pkg/apis/istio/v1alpha3"
	"github.com/knative/pkg/kmeta"
	"github.com/knative/serving/pkg/apis/networking/v1alpha1"
	"github.com/knative/serving/pkg/reconciler"
	"github.com/knative/serving/pkg/reconciler/v1alpha1/clusteringress/resources/names"
	"github.com/knative/serving/pkg/system"
)

// MakeVirtualService creates an Istio VirtualService as network programming.
// Such VirtualService specifies which Gateways and Hosts that it applies to,
// as well as the routing rules.
func MakeVirtualService(ci *v1alpha1.ClusterIngress) *v1alpha3.VirtualService {
	return &v1alpha3.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:            names.VirtualService(ci),
			Namespace:       system.Namespace,
			Labels:          map[string]string{"clusterIngress": ci.Name},
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(ci)},
		},
		Spec: *makeVirtualServiceSpec(ci),
	}
}

func makeVirtualServiceSpec(ci *v1alpha1.ClusterIngress) *v1alpha3.VirtualServiceSpec {
	spec := v1alpha3.VirtualServiceSpec{
		// We want to connect to two Gateways: the Knative shared
		// Gateway, and the 'mesh' Gateway.  The former provides
		// access from outside of the cluster, and the latter provides
		// access for services from inside the cluster.
		Gateways: []string{
			names.K8sGatewayFullname,
			"mesh",
		},
		Hosts: getHosts(ci),
	}

	for _, rule := range ci.Spec.Rules {
		hosts := rule.Hosts
		for _, p := range rule.HTTP.Paths {
			spec.Http = append(spec.Http, *makeVirtualServiceRoute(hosts, &p))
		}
	}
	return &spec
}

func makePortSelector(ios intstr.IntOrString) v1alpha3.PortSelector {
	if ios.Type == intstr.Int {
		return v1alpha3.PortSelector{
			Number: uint32(ios.IntValue()),
		}
	}
	return v1alpha3.PortSelector{
		Name: ios.String(),
	}
}

func makeVirtualServiceRoute(hosts []string, http *v1alpha1.HTTPClusterIngressPath) *v1alpha3.HTTPRoute {
	matches := []v1alpha3.HTTPMatchRequest{}
	for _, host := range hosts {
		matches = append(matches, makeMatch(host, http.Path))
	}
	weights := []v1alpha3.DestinationWeight{}
	for _, split := range http.Splits {
		weights = append(weights, v1alpha3.DestinationWeight{
			Destination: v1alpha3.Destination{
				Host: reconciler.GetK8sServiceFullname(
					split.Backend.ServiceName, split.Backend.ServiceNamespace),
				Port: makePortSelector(split.Backend.ServicePort),
			},
			Weight: split.Percent,
		})
	}
	return &v1alpha3.HTTPRoute{
		Match:   matches,
		Route:   weights,
		Timeout: timeoutToString(http.Timeout.Duration),
		Retries: &v1alpha3.HTTPRetry{
			Attempts:      http.Retries.Attempts,
			PerTryTimeout: timeoutToString(http.Retries.PerTryTimeout.Duration),
		},
		AppendHeaders: http.AppendHeaders,
	}
}

func makeMatch(host string, pathRegExp string) v1alpha3.HTTPMatchRequest {
	match := v1alpha3.HTTPMatchRequest{
		Authority: &istiov1alpha1.StringMatch{
			Exact: host,
		},
	}
	// Empty pathRegExp is considered match all path. We only need to
	// consider pathRegExp when it's non-empty.
	if pathRegExp != "" {
		match.Uri = &istiov1alpha1.StringMatch{
			Regex: pathRegExp,
		}
	}
	return match
}

func getHosts(ci *v1alpha1.ClusterIngress) []string {
	hosts := make(map[string]interface{})
	unique := []string{}
	for _, rule := range ci.Spec.Rules {
		for _, h := range rule.Hosts {
			if _, existed := hosts[h]; !existed {
				hosts[h] = true
				unique = append(unique, h)
			}
		}
	}
	// Sort the names to give a deterministic ordering.
	sort.Strings(unique)
	return unique
}

func timeoutToString(timeout time.Duration) string {
	return fmt.Sprintf("%ds", int(timeout.Truncate(time.Second).Seconds()))
}
