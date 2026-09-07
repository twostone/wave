/*
Copyright 2018 Pusher Ltd. and Wave Contributors

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

package utils

import (
	"time"

	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ConsumeReconciles waits for the given number of reconciles of the object to finish
func ConsumeReconciles(started <-chan reconcile.Request, finished <-chan reconcile.Request, obj client.Object, timeout time.Duration, times int) {
	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		},
	}
	for range times {
		gomega.Eventually(started, timeout).Should(gomega.Receive(gomega.Equal(request)))
		gomega.Eventually(finished, timeout).Should(gomega.Receive(gomega.Equal(request)))
	}
}

// DrainReconciles consumes reconciles until none are triggered anymore. The
// channels are unbuffered, so the reconciler blocks until they are read.
func DrainReconciles(started <-chan reconcile.Request, finished <-chan reconcile.Request) {
	for {
		select {
		case <-started:
			<-finished
		case <-time.After(time.Millisecond * 100):
			return
		}
	}
}
