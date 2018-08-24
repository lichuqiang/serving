/*
Copyright 2018 The Knative Authors.

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

package v1alpha1

import (
	"encoding/json"

	sapis "github.com/knative/serving/pkg/apis"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced

// ClusterIngress is a collection of rules that allow inbound connections to reach the
// endpoints defined by a backend. An ClusterIngress can be configured to give services
// externally-reachable urls, load balance traffic offer name based virtual hosting etc.
type ClusterIngress struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of the ClusterIngress.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status
	// +optional
	Spec ClusterIngressSpec `json:"spec,omitempty"`

	// Status is the current state of the ClusterIngress.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status
	// +optional
	Status ClusterIngressStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterIngressList is a collection of ClusterIngress.
type ClusterIngressList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of ClusterIngress.
	Items []ClusterIngress `json:"items"`
}

func (ci *ClusterIngress) GetGeneration() int64 {
	return ci.Spec.Generation
}

func (ci *ClusterIngress) SetGeneration(generation int64) {
	ci.Spec.Generation = generation
}

func (ci *ClusterIngress) GetSpecJSON() ([]byte, error) {
	return json.Marshal(ci.Spec)
}

func (ci *ClusterIngress) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("ClusterIngress")
}

// ClusterIngressSpec describes the ClusterIngress the user wishes to exist.
type ClusterIngressSpec struct {
	// TODO: Generation does not work correctly with CRD. They are scrubbed
	// by the APIserver (https://github.com/kubernetes/kubernetes/issues/58778)
	// So, we add Generation here. Once that gets fixed, remove this and use
	// ObjectMeta.Generation instead.
	// +optional
	Generation int64 `json:"generation,omitempty"`

	// A default backend capable of servicing requests that don't match any
	// rule. At least one of 'backend' or 'rules' must be specified. This field
	// is optional to allow the loadbalancer controller or defaulting logic to
	// specify a global default.
	// +optional
	Backend *ClusterIngressBackend `json:"backend,omitempty"`

	// TLS configuration. Currently the ClusterIngress only supports a single TLS
	// port, 443. If multiple members of this list specify different hosts, they
	// will be multiplexed on the same port according to the hostname specified
	// through the SNI TLS extension, if the ingress controller fulfilling the
	// ingress supports SNI.
	// +optional
	TLS []ClusterIngressTLS `json:"tls,omitempty"`

	// A list of host rules used to configure the ClusterIngress. If unspecified, or
	// no rule matches, all traffic is sent to the default backend.
	// +optional
	Rules []ClusterIngressRule `json:"rules,omitempty"`
	// TODO: Add the ability to specify load-balancer IP through claims
}

// ClusterIngressTLS describes the transport layer security associated with an ClusterIngress.
type ClusterIngressTLS struct {
	// Hosts are a list of hosts included in the TLS certificate. The values in
	// this list must match the name/s used in the tlsSecret. Defaults to the
	// wildcard host setting for the loadbalancer controller fulfilling this
	// ClusterIngress, if left unspecified.
	// +optional
	Hosts []string `json:"hosts,omitempty"`
	// SecretName is the name of the secret used to terminate SSL traffic on 443.
	// Field is left optional to allow SSL routing based on SNI hostname alone.
	// If the SNI host in a listener conflicts with the "Host" header field used
	// by an ClusterIngressRule, the SNI host is used for termination and value of the
	// Host header is used for routing.
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

// ConditionType represents a ClusterIngress condition value
const (
	// ClusterIngressConditionReady is set when the clusterIngress is configured
	// and has a ready VirtualService.
	ClusterIngressConditionReady = sapis.ConditionReady

	// ClusterIngressConditionVirtualServiceReady is set when the ClusterIngress's underlying
	// VirtualService have reported being Ready.
	ClusterIngressConditionVirtualServiceReady sapis.ConditionType = "VirtualServiceReady"
)

var clusterIngressCondSet = sapis.NewLivingConditionSet(ClusterIngressConditionVirtualServiceReady)

// ClusterIngressStatus describe the current state of the ClusterIngress.
type ClusterIngressStatus struct {
	// +optional
	Conditions sapis.Conditions `json:"conditions,omitempty"`
	// LoadBalancer contains the current status of the load-balancer.
	// +optional
	LoadBalancer *v1.LoadBalancerStatus `json:"loadBalancer,omitempty"`
}

// LoadBalancerStatus represents the status of a load-balancer.
type LoadBalancerStatus struct {
	// Ingress is a list containing ingress points for the load-balancer.
	// Traffic intended for the service should be sent to these ingress points.
	// +optional
	Ingress []LoadBalancerIngress `json:"ingress,omitempty" protobuf:"bytes,1,rep,name=ingress"`
}

// LoadBalancerIngress represents the status of a load-balancer ingress point:
// traffic intended for the service should be sent to an ingress point.
type LoadBalancerIngress struct {
	// IP is set for load-balancer ingress points that are IP based
	// (typically GCE or OpenStack load-balancers)
	// +optional
	IP string `json:"ip,omitempty" protobuf:"bytes,1,opt,name=ip"`

	// Domain is set for load-balancer ingress points that are DNS based
	// (typically AWS load-balancers)
	// +optional
	Domain string `json:"hostname,omitempty" protobuf:"bytes,2,opt,name=hostname"`

	// DomainInternal is set if there is a cluster-local DNS name to access the Ingress.
	DomainInternal string `json:"hostname,omitempty"`
}

func (cis *ClusterIngressStatus) GetCondition(t sapis.ConditionType) *sapis.Condition {
	return clusterIngressCondSet.Manage(cis).GetCondition(t)
}

func (cis *ClusterIngressStatus) InitializeConditions() {
	clusterIngressCondSet.Manage(cis).InitializeConditions()
}

func (cis *ClusterIngressStatus) MarkVirtualServiceReady() {
	clusterIngressCondSet.Manage(cis).MarkTrue(ClusterIngressConditionVirtualServiceReady)
}

// GetConditions returns the Conditions array. This enables generic handling of
// conditions by implementing the sapis.Conditions interface.
func (cis *ClusterIngressStatus) GetConditions() sapis.Conditions {
	return cis.Conditions
}

// SetConditions sets the Conditions array. This enables generic handling of
// conditions by implementing the sapis.Conditions interface.
func (cis *ClusterIngressStatus) SetConditions(conditions sapis.Conditions) {
	cis.Conditions = conditions
}

// ClusterIngressRule represents the rules mapping the paths under a specified host to
// the related backend services. Incoming requests are first evaluated for a host
// match, then routed to the backend associated with the matching ClusterIngressRuleValue.
type ClusterIngressRule struct {
	// Host is the fully qualified domain name of a network host, as defined
	// by RFC 3986. Note the following deviations from the "host" part of the
	// URI as defined in the RFC:
	// 1. IPs are not allowed. Currently an ClusterIngressRuleValue can only apply to the
	//	  IP in the Spec of the parent ClusterIngress.
	// 2. The `:` delimiter is not respected because ports are not allowed.
	//	  Currently the port of an ClusterIngress is implicitly :80 for http and
	//	  :443 for https.
	// Both these may change in the future.
	// Incoming requests are matched against the host before the ClusterIngressRuleValue.
	// If the host is unspecified, the ClusterIngress routes all traffic based on the
	// specified ClusterIngressRuleValue.
	// +optional
	Hosts []string `json:"hosts,omitempty"`
	// ClusterIngressRuleValue represents a rule to route requests for this ClusterIngressRule.
	// If unspecified, the rule defaults to a http catch-all. Whether that sends
	// just traffic matching the host to the default backend or all traffic to the
	// default backend, is left to the controller fulfilling the ClusterIngress. Http is
	// currently the only supported ClusterIngressRuleValue.
	// +optional
	ClusterIngressRuleValue `json:",inline,omitempty"`
}

// ClusterIngressRuleValue represents a rule to apply against incoming requests. If the
// rule is satisfied, the request is routed to the specified backend. Currently
// mixing different types of rules in a single ClusterIngress is disallowed, so exactly
// one of the following must be set.
type ClusterIngressRuleValue struct {
	//TODO:
	// 1. Consider renaming this resource and the associated rules so they
	// aren't tied to ClusterIngress. They can be used to route intra-cluster traffic.
	// 2. Consider adding fields for ingress-type specific global options
	// usable by a loadbalancer, like http keep-alive.

	// +optional
	HTTP *HTTPClusterIngressRuleValue `json:"http,omitempty"`
}

// HTTPClusterIngressRuleValue is a list of http selectors pointing to backends.
// In the example: http://<host>/<path>?<searchpart> -> backend where
// where parts of the url correspond to RFC 3986, this resource will be used
// to match against everything after the last '/' and before the first '?'
// or '#'.
type HTTPClusterIngressRuleValue struct {
	// A collection of paths that map requests to backends.
	Paths []HTTPClusterIngressPath `json:"paths"`
	// TODO: Consider adding fields for ingress-type specific global
	// options usable by a loadbalancer, like http keep-alive.
}

// HTTPClusterIngressPath associates a path regex with a backend. Incoming urls matching
// the path are forwarded to the backend.
type HTTPClusterIngressPath struct {
	// Path is an extended POSIX regex as defined by IEEE Std 1003.1,
	// (i.e this follows the egrep/unix syntax, not the perl syntax)
	// matched against the path of an incoming request. Currently it can
	// contain characters disallowed from the conventional "path"
	// part of a URL as defined by RFC 3986. Paths must begin with
	// a '/'. If unspecified, the path defaults to a catch all sending
	// traffic to the backend.
	// +optional
	Path string `json:"path,omitempty"`

	// Backend defines the referenced service endpoint to which the traffic
	// will be forwarded to.
	//
	// @knative: s/Backend *ClusterIngressBackend/Backends []ClusterIngressBackendSplit
	Backends []ClusterIngressBackendSplit `json:"backends"`

	// Additional HTTP headers to add before forwarding a request to the
	// destination service.
	//
	// @knative
	AppendHeaders map[string]string `json:"appendHeaders,omitempty"`

	// Timeout for HTTP requests.
	//
	// @knative
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Retry policy for HTTP requests.
	//
	// @knative
	Retries *HTTPRetry `json:"retries,omitempty"`
}

// HTTPRetry describes the retry policy to use when a HTTP request fails.
type HTTPRetry struct {
	// REQUIRED. Number of retries for a given request. The interval
	// between retries will be determined automatically (25ms+). Actual
	// number of retries attempted depends on the httpReqTimeout.
	Attempts int `json:"attempts"`

	// Timeout per retry attempt for a given request. format: 1h/1m/1s/1ms. MUST BE >=1ms.
	PerTryTimeout *metav1.Duration `json:"perTryTimeout"`
}

// ClusterIngressBackend describes all endpoints for a given service and port.
type ClusterIngressBackend struct {
	// Specifies the namespace of the referenced service.
	ServiceNamespace string `json:"serviceNamespace"`

	// Specifies the name of the referenced service.
	ServiceName string `json:"serviceName"`

	// Specifies the port of the referenced service.
	ServicePort intstr.IntOrString `json:"servicePort"`
}

// ClusterIngressBackend describes all endpoints for a given service and port.
//
// @knative
type ClusterIngressBackendSplit struct {
	// Specifies the backend receiving the traffic split.
	Backend *ClusterIngressBackend `json:"backend"`

	// Specifies the split percentage, a number between 0 and 100.  Defaults to 100.
	Percent int `json:"percent,omitempty"`
}

// TODO(tcnghia): Check that ClusterIngress can be validated, can be defaulted, and has immutable fields.
// var _ apis.Validatable = (*ClusterIngress)(nil)
// var _ apis.Defaultable = (*ClusterIngress)(nil)
// var _ apis.Immutable = (*ClusterIngress)(nil)
