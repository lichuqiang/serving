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
	"sort"
	"testing"

	netv1alpha1 "github.com/knative/serving/pkg/apis/networking/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createIngress(name string) *netv1alpha1.ClusterIngress {
	return &netv1alpha1.ClusterIngress{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func TestSortIngressByName(t *testing.T) {
	names := []string{"ingress-c", "ingress-a", "ingress-b"}
	ingresses := []*netv1alpha1.ClusterIngress{}
	for _, name := range names {
		ingresses = append(ingresses, createIngress(name))
	}

	sort.Strings(names)
	sort.Sort(ingressByName(ingresses))

	for index, ingress := range ingresses {
		if names[index] != ingress.GetName() {
			t.Errorf("Unexpected ingress name, want %s, got %s", names[index], ingress.GetName())
		}
	}
}
