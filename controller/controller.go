/*
Copyright 2016 The Kubernetes Authors.
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

package controller

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
	core_v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/runtime"
	"k8s.io/client-go/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

const (
	component = "podVolumeMultiplexer"
)

type PodVolumeController struct {
	client kubernetes.Interface

	podSource       cache.ListerWatcher
	podController   *cache.Controller
	claimSource     cache.ListerWatcher
	claimController *cache.Controller

	podStore      cache.Store
	claimStore    cache.Store
	eventRecorder record.EventRecorder
}

func NewPodVolumeController(client kubernetes.Interface, resyncPeriod time.Duration, namespace string) (*PodVolumeController, error) {
	broadcaster := record.NewBroadcaster()
	broadcaster.StartRecordingToSink(&core_v1.EventSinkImpl{Interface: client.Core().Events(v1.NamespaceAll)})
	var eventRecorder record.EventRecorder
	out, err := exec.Command("hostname").Output()
	if err != nil {
		glog.Errorf("Error getting hostname for specifying it as source of events: %v", err)
		eventRecorder = broadcaster.NewRecorder(v1.EventSource{Component: component})
	} else {
		eventRecorder = broadcaster.NewRecorder(v1.EventSource{Component: fmt.Sprintf("%s-%s", component, strings.TrimSpace(string(out)))})
	}

	controller := &PodVolumeController{
		client:        client,
		eventRecorder: eventRecorder,
	}
	// pod
	controller.podSource = &cache.ListWatch{
		ListFunc: func(options api.ListOptions) (runtime.Object, error) {
			var out v1.ListOptions
			v1.Convert_api_ListOptions_To_v1_ListOptions(&options, &out, nil)
			return client.Core().Pods(namespace).List(out)
		},
		WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
			var out v1.ListOptions
			v1.Convert_api_ListOptions_To_v1_ListOptions(&options, &out, nil)
			return client.Core().Pods(namespace).Watch(out)
		},
	}

	controller.podStore, controller.podController = cache.NewInformer(
		controller.podSource,
		&v1.Pod{},
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    addPod,
			UpdateFunc: updatePod,
			DeleteFunc: deletePod,
		},
	)
	// claim
	controller.claimSource = &cache.ListWatch{
		ListFunc: func(options api.ListOptions) (runtime.Object, error) {
			var out v1.ListOptions
			v1.Convert_api_ListOptions_To_v1_ListOptions(&options, &out, nil)
			return client.Core().PersistentVolumeClaims(namespace).List(out)
		},
		WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
			var out v1.ListOptions
			v1.Convert_api_ListOptions_To_v1_ListOptions(&options, &out, nil)
			return client.Core().PersistentVolumeClaims(namespace).Watch(out)
		},
	}

	controller.claimStore, controller.claimController = cache.NewInformer(
		controller.claimSource,
		&v1.PersistentVolumeClaim{},
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    addPVC,
			UpdateFunc: updatePVC,
			DeleteFunc: deletePVC,
		},
	)

	return controller, nil
}

// pod
func addPod(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		glog.Errorf("Expected Pod but received %+v", obj)
		return
	}

	glog.Infof("add pod: %s", pod.Name)
	vols := pod.Spec.Volumes
	for _, vol := range vols {
		if vol.VolumeSource.PersistentVolumeClaim != nil {
			glog.Infof("pod PVC: %s", vol.VolumeSource.PersistentVolumeClaim.ClaimName)
		}

	}
}

func updatePod(oldObj, newObj interface{}) {

}

func deletePod(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		glog.Errorf("Expected Pod but received %+v", obj)
		return
	}

	glog.Infof("delete pod: %s", pod.Name)
}

// claims
func addPVC(obj interface{}) {
	pvc, ok := obj.(*v1.PersistentVolumeClaim)
	if !ok {
		glog.Errorf("Expected PVC but received %+v", obj)
		return
	}

	glog.Infof("add pvc: %s", pvc.Name)
}

func updatePVC(oldObj, newObj interface{}) {

}

func deletePVC(obj interface{}) {
	pvc, ok := obj.(*v1.PersistentVolumeClaim)
	if !ok {
		glog.Errorf("Expected PVC but received %+v", obj)
		return
	}

	glog.Infof("delete pvc: %s", pvc.Name)
}

func (ctrl *PodVolumeController) Run(stopCh <-chan struct{}) {
	glog.Info("Starting pod volume controller!")
	go ctrl.podController.Run(stopCh)
	go ctrl.claimController.Run(stopCh)
	<-stopCh
}
