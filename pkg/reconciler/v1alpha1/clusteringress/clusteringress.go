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

package clusteringress

import (
	"context"
	"reflect"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"github.com/knative/pkg/apis/istio/v1alpha3"
	istioinformers "github.com/knative/pkg/client/informers/externalversions/istio/v1alpha3"
	istiolisters "github.com/knative/pkg/client/listers/istio/v1alpha3"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/logging"
	"github.com/knative/serving/pkg/apis/networking/v1alpha1"
	informers "github.com/knative/serving/pkg/client/informers/externalversions/networking/v1alpha1"
	listers "github.com/knative/serving/pkg/client/listers/networking/v1alpha1"
	"github.com/knative/serving/pkg/reconciler"
	"github.com/knative/serving/pkg/reconciler/v1alpha1/clusteringress/resources"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

const controllerAgentName = "clusteringress-controller"

// Reconciler implements controller.Reconciler for Service resources.
type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	clusterIngressLister listers.ClusterIngressLister
	serviceLister        corev1listers.ServiceLister
	virtualServiceLister istiolisters.VirtualServiceLister
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController initializes the controller and is called by the generated code
// Registers eventhandlers to enqueue events
func NewController(
	opt reconciler.Options,
	clusterIngressInformer informers.ClusterIngressInformer,
	serviceInformer corev1informers.ServiceInformer,
	virtualServiceInformer istioinformers.VirtualServiceInformer,
) *controller.Impl {

	c := &Reconciler{
		Base:                 reconciler.NewBase(opt, controllerAgentName),
		clusterIngressLister: clusterIngressInformer.Lister(),
		serviceLister:        serviceInformer.Lister(),
		virtualServiceLister: virtualServiceInformer.Lister(),
	}
	impl := controller.NewImpl(c, c.Logger, "ClusterIngresses")

	c.Logger.Info("Setting up event handlers")
	clusterIngressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    impl.Enqueue,
		UpdateFunc: controller.PassNew(impl.Enqueue),
		DeleteFunc: impl.Enqueue,
	})

	virtualServiceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("ClusterIngress")),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    impl.EnqueueControllerOf,
			UpdateFunc: controller.PassNew(impl.EnqueueControllerOf),
		},
	})

	serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("ClusterIngress")),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    impl.EnqueueControllerOf,
			UpdateFunc: controller.PassNew(impl.EnqueueControllerOf),
		},
	})
	return impl
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Service resource
// with the current status of the resource.
func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.Logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	// Get the ClusterIngress resource with this name.
	//
	// Using namespace 'default' since ClusterIngress is a cluster-scoped resource.
	original, err := c.clusterIngressLister.Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		c.Logger.Errorf("clusteringress %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		return err
	}
	// Don't modify the informers copy
	ci := original.DeepCopy()

	// Reconcile this copy of the ClusterIngress and then write back any status
	// updates regardless of whether the reconciliation errored out.
	err = c.reconcile(ctx, ci)
	if equality.Semantic.DeepEqual(original.Status, ci.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err := c.updateStatus(ctx, ci); err != nil {
		c.Logger.Warn("Failed to update clusterIngress status", zap.Error(err))
		c.Recorder.Eventf(ci, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for ClusterIngress %q: %v", ci.Name, err)
		return err
	}
	return err
}

// Update the Status of the route.  Caller is responsible for checking
// for semantic differences before calling.
func (c *Reconciler) updateStatus(ctx context.Context, ci *v1alpha1.ClusterIngress) (*v1alpha1.ClusterIngress, error) {
	existing, err := c.clusterIngressLister.Get(ci.Name)
	if err != nil {
		return nil, err
	}
	// If there's nothing to update, just return.
	if reflect.DeepEqual(existing.Status, ci.Status) {
		return existing, nil
	}
	existing.Status = ci.Status
	// TODO: for CRD there's no updatestatus, so use normal update.
	updated, err := c.ServingClientSet.NetworkingV1alpha1().ClusterIngresses().Update(existing)
	if err != nil {
		return nil, err
	}

	c.Recorder.Eventf(ci, corev1.EventTypeNormal, "Updated", "Updated status for clusterIngress %q", ci.Name)
	return updated, nil
}

func (c *Reconciler) reconcile(ctx context.Context, ci *v1alpha1.ClusterIngress) error {
	logger := logging.FromContext(ctx)
	ci.Status.InitializeConditions()
	vs := resources.MakeVirtualService(ci)

	logger.Info("Creating/Updating placeholder k8s services")
	if err := c.reconcilePlaceholderService(ctx, ci); err != nil {
		return err
	}
	logger.Infof("Reconciling clusterIngress :%v", ci)
	logger.Info("Creating/Updating VirtualService")
	if err := c.reconcileVirtualService(ctx, ci, vs); err != nil {
		return err
	}
	ci.Status.MarkVirtualServiceReady()
	logger.Info("ClusterIngress successfully synced")
	return nil
}

func (c *Reconciler) reconcilePlaceholderService(ctx context.Context, clusterIngress *v1alpha1.ClusterIngress) error {
	logger := logging.FromContext(ctx)

	desiredServices := resources.MakeK8sServices(clusterIngress)
	for _, desiredService := range desiredServices {
		name := desiredService.Name
		ns := desiredService.Namespace
		service, err := c.serviceLister.Services(ns).Get(name)
		if apierrs.IsNotFound(err) {
			// Doesn't exist, create it.
			service, err = c.KubeClientSet.CoreV1().Services(ns).Create(&desiredService)
			if err != nil {
				logger.Error("Failed to create service %v+", zap.Error(err))
				c.Recorder.Eventf(clusterIngress, corev1.EventTypeWarning, "CreationFailed",
					"Failed to create service %s/%s: %v", ns, name, err)
				return err
			}
			logger.Infof("Created service %s/%s", ns, name)
			c.Recorder.Eventf(clusterIngress, corev1.EventTypeNormal, "Created", "Created service %s/%s", ns, name)
		} else if err != nil {
			return err
		} else {
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
	}
	// TODO(mattmoor): This is where we'd look at the state of the Service and
	// reflect any necessary state into the Route.
	return nil
}

func (c *Reconciler) reconcileVirtualService(ctx context.Context, ci *v1alpha1.ClusterIngress,
	desired *v1alpha3.VirtualService) error {
	logger := logging.FromContext(ctx)
	ns := desired.Namespace
	name := desired.Name

	vs, err := c.virtualServiceLister.VirtualServices(ns).Get(name)
	if apierrs.IsNotFound(err) {
		vs, err = c.SharedClientSet.NetworkingV1alpha3().VirtualServices(ns).Create(desired)
		if err != nil {
			logger.Error("Failed to create VirtualService", zap.Error(err))
			c.Recorder.Eventf(ci, corev1.EventTypeWarning, "CreationFailed",
				"Failed to create VirtualService %q: %v", name, err)
			return err
		}
		c.Recorder.Eventf(ci, corev1.EventTypeNormal, "Created",
			"Created VirtualService %q", desired.Name)
	} else if err != nil {
		return err
	} else if !equality.Semantic.DeepEqual(vs.Spec, desired.Spec) {
		vs.Spec = desired.Spec
		vs, err = c.SharedClientSet.NetworkingV1alpha3().VirtualServices(ns).Update(vs)
		if err != nil {
			logger.Error("Failed to update VirtualService", zap.Error(err))
			return err
		}
	}
	return err
}
