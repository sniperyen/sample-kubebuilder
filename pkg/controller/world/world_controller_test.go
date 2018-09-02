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
	"testing"
	"time"

	hellov1beta1 "hello/pkg/apis/hello/v1beta1"

	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var c client.Client

const timeout = time.Second * 5

func newNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func newConfigmap(name, namespace string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}, Data: data}
}

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// init
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	recFn, _ := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// mock some data
	n1 := newNamespace("n1")
	n2 := newNamespace("n2")
	c1 := newConfigmap("c1", "default", map[string]string{"a": "b"})

	tables := []k8sruntime.Object{n1, n2, c1}
	for _, ta := range tables {
		err = c.Create(context.TODO(), ta)
		if apierrors.IsInvalid(err) {
			t.Logf("failed to create object, got an invalid object error: %v", err)
			return
		}
		g.Expect(err).NotTo(gomega.HaveOccurred())
		defer c.Delete(context.TODO(), ta)
	}

	world := &hellov1beta1.World{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"},
		Spec:       hellov1beta1.WorldSpec{Name: c1.GetName(), Namespace: c1.GetNamespace()},
	}
	g.Eventually(func() error { return c.Create(context.TODO(), world) }, timeout).
		Should(gomega.Succeed())

	nsList := []*corev1.Namespace{n1, n2}
	for _, ns := range nsList {
		cm := &corev1.ConfigMap{}
		cmKey := types.NamespacedName{Name: c1.GetName(), Namespace: ns.GetName()}
		g.Eventually(func() error {
			err := c.Get(context.TODO(), cmKey, cm)
			return err
		}, timeout).Should(gomega.Succeed())

		g.Expect(cm.Data).To(gomega.Equal(c1.Data))
		g.Expect(cm.GetName()).To(gomega.Equal(c1.GetName()))
		g.Expect(cm.GetNamespace()).To(gomega.Equal(ns.Name))
	}

	g.Eventually(func() hellov1beta1.WorldStatus {
		worldNew := &hellov1beta1.World{}
		wdKey := types.NamespacedName{Name: world.GetName(), Namespace: world.GetNamespace()}
		c.Get(context.TODO(), wdKey, worldNew)
		return worldNew.Status
	}, timeout).Should(gomega.Equal(hellov1beta1.WorldStatus{Phase: "Success"}))
}
