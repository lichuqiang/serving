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
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	corev1listers "k8s.io/client-go/listers/core/v1"

	"github.com/knative/serving/pkg/reconciler/v1alpha1/clusteringress/prober/status"
	"github.com/knative/serving/pkg/system"
)

const (
	probeTimeout = 5 * time.Second
	// TODO(lichuqiang): move these to public places.
	ingressGatewayService   = "knative-ingressgateway"
	ingressGatewayNamespace = "istio-system"
)

// Prober helps to check the health state of given ClusterIngress(generation).
type prober struct {
	endpointsLister corev1listers.EndpointsLister
	httpClient      http.Client
}

// newProber returns with a prober instance.
func newProber(endpointsLister corev1listers.EndpointsLister) *prober {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: probeTimeout,
		}).DialContext,
	}
	client := http.Client{Transport: transport}

	return &prober{
		endpointsLister: endpointsLister,
		httpClient:      client,
	}
}

func (p *prober) probe(ingress string, generation int64) (res status.Result) {
	res = status.Result{
		Ready:  false,
		Reason: "probe failed",
	}

	// Fetch the enpoints of ingress gateway.
	endpoints, err := p.endpointsLister.Endpoints(ingressGatewayNamespace).Get(ingressGatewayService)
	if err != nil {
		res.Message = fmt.Sprintf("failed to get ingress gateway endpoints: %v", err)
		return
	}
	// Range over the endpoints for target url of the gateway instances.
	targets := []string{}
	for _, subset := range endpoints.Subsets {
		instanceIP, instancePort := "", ""
		for _, address := range subset.Addresses {
			if address.IP != "" {
				instanceIP = address.IP
				break
			}
		}
		for _, port := range subset.Ports {
			if port.Name == "http2" {
				instancePort = strconv.Itoa(int(port.Port))
				break
			}
		}

		if instanceIP == "" || instancePort == "" {
			res.Message = fmt.Sprintf("failed to get ip/port from endpoint: %v", subset)
			return
		}
		targets = append(targets, fmt.Sprintf("%s:%s", instanceIP, instancePort))
	}

	if len(targets) == 0 {
		res.Message = fmt.Sprintf("no available endpoint for ingress gateway service: %s/%s",
			ingressGatewayNamespace, ingressGatewayService)
		return
	}

	// Host header is in format of <ingress-name>-<generation>.<system-namespace>
	hostHeader := fmt.Sprintf("%s-%v.%s", ingress, generation, system.Namespace)

	for _, targetUrl := range targets {
		req, err := http.NewRequest(http.MethodGet, targetUrl, nil)
		if err != nil {
			// This should never happen
			res.Message = fmt.Sprintf("failed to construct probe request: %v", err)
			return
		}
		req.Header["Host"] = []string{hostHeader}

		resp, err := p.httpClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			res.Message = fmt.Sprintf("probe request error: %v, response: %v", err, resp)
			return
		}
	}

	res = status.Result{
		Ready: true,
	}

	return
}
