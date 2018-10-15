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

import netv1alpha1 "github.com/knative/serving/pkg/apis/networking/v1alpha1"

// ingressByName implements sort.Interface to allow ingress objects to be sorted by name.
type ingressByName []*netv1alpha1.ClusterIngress

// Len implements sort.Interface of Len.
func (ibn ingressByName) Len() int {
	return len(ibn)
}

// Less implements sort.Interface of Less.
func (ibn ingressByName) Less(i, j int) bool {
	return ibn[i].GetName() < ibn[j].GetName()
}

// Swap implements sort.Interface of Swap.
func (ibn ingressByName) Swap(i, j int) {
	ibn[i], ibn[j] = ibn[j], ibn[i]
}
