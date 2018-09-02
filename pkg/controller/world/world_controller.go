/*

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

package world

import (
	"context"
	hellov1beta1 "hello/pkg/apis/hello/v1beta1"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"log"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new World Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this hello.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileWorld{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("world-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to World
	err = c.Watch(&source.Kind{Type: &hellov1beta1.World{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &hellov1beta1.World{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &hellov1beta1.World{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileWorld{}

// ReconcileWorld reconciles a World object
type ReconcileWorld struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a World object and makes changes based on the state read
// and what is in the World.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hello.k8s.io,resources=worlds,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileWorld) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	var (
		result     = reconcile.Result{}
		err        error
		worldError error
	)

	log.Println("Fetch the World world")
	world := &hellov1beta1.World{}
	err = r.Get(context.TODO(), request.NamespacedName, world)
	if err != nil {
		if errors.IsNotFound(err) {
			return result, nil
		}
		return result, err
	}

	defer func() {
		worldNew := world.DeepCopy()
		if worldError != nil {
			worldNew.Status.Phase = "Error"
			worldNew.Status.Message = worldError.Error()
		} else {
			worldNew.Status.Phase = "Success"
			worldNew.Status.Message = ""
		}
		err = r.Update(context.TODO(), worldNew)
		if err != nil {
			glog.Errorf("Update world; err: %v", err)
			return
		}
	}()

	log.Println("Fetch configmap defined in World")
	cm := &corev1.ConfigMap{}
	cmKey := types.NamespacedName{Name: world.Spec.Name, Namespace: world.Spec.Namespace}
	err = r.Get(context.TODO(), cmKey, cm)
	if err != nil {
		glog.Errorf("Get configmap %s/%s; err: %v", cmKey.Namespace, cmKey.Name, err)
		worldError = err
		return result, err
	}

	log.Println("List the namespaces")
	nsList := &corev1.NamespaceList{}
	err = r.List(context.TODO(), &client.ListOptions{}, nsList)
	if err != nil {
		glog.Errorf("List the namespaces; err: %v", err)
		worldError = err
		return result, err
	}

	for _, ns := range nsList.Items {
		log.Printf("Copy configmap %s from %s to %s", cm.Name, cm.Namespace, ns.Name)
		if ns.GetName() == cm.GetNamespace() {
			log.Println("Skip for the same namespace ...")
			continue
		}

		cmNew := &corev1.ConfigMap{}
		err = r.Get(context.TODO(), types.NamespacedName{Name: world.Spec.Name, Namespace: ns.Name}, cmNew)
		if err != nil && errors.IsNotFound(err) {
			cmNew := cm.DeepCopy()
			cmNew.Namespace = ns.Name
			cmNew.ResourceVersion = ""
			cmNew.DeletionTimestamp = nil
			if err := controllerutil.SetControllerReference(world, cmNew, r.scheme); err != nil {
				glog.Errorf("SetControllerReference; err: %v", err)
				worldError = err
				return result, err
			}

			err = r.Create(context.TODO(), cmNew)
			if err != nil {
				glog.Errorf("Copy configmap %s from %s to %s; err: %v", cm.Name, cm.Namespace, cmNew.Namespace, err)
				worldError = err
				return result, err
			}
		}
	}

	worldError = err
	return result, err
}
