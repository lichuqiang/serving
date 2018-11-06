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
	"time"

	"github.com/knative/serving/pkg/reconciler/v1alpha1/clusteringress/prober/status"
	"github.com/knative/serving/pkg/reconciler/v1alpha1/clusteringress/resources/names"
	"github.com/knative/serving/pkg/system"
)

var probeTimeout = 5 * time.Second

// Prober helps to check the health state of given ClusterIngress(generation).
type prober struct {
	httpClient http.Client
}

// newProber returns with a prober instance.
func newProber() *prober {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: probeTimeout,
		}).DialContext,
	}
	client := http.Client{Transport: transport}

	return &prober{
		httpClient: client,
	}
}

func (p *prober) probe(ingress string, generation int64) status.Result {
	probeUrl := names.K8sGatewayServiceFullname
	// Host header is in format of <ingress-name>-<generation>.<system-namespace>
	hostHeader := fmt.Sprintf("%s-%v.%s", ingress, generation, system.Namespace)

	req, err := http.NewRequest(http.MethodGet, probeUrl, nil)
	if err != nil {
		// This should never happen
		return status.Result{}
	}
	req.Header["Host"] = []string{hostHeader}

	res := status.Result{
		Ready: true,
	}
	resp, err := p.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		res = status.Result{
			Ready:   false,
			Reason:  "probe request failed",
			Message: fmt.Sprintf("probe request error: %v, response: %v", err, resp),
		}
	}

	return res
}
