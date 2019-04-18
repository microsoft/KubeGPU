/*
Copyright 2014 The Kubernetes Authors.

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

package factory

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	fakeV1 "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
	"k8s.io/kubernetes/pkg/scheduler/algorithm"
	"k8s.io/kubernetes/pkg/scheduler/algorithm/predicates"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/api"
	latestschedulerapi "k8s.io/kubernetes/pkg/scheduler/api/latest"
	schedulerinternalcache "k8s.io/kubernetes/pkg/scheduler/internal/cache"
	internalqueue "k8s.io/kubernetes/pkg/scheduler/internal/queue"
	schedulernodeinfo "k8s.io/kubernetes/pkg/scheduler/nodeinfo"
	"k8s.io/kubernetes/pkg/scheduler/util"
)

const (
	disablePodPreemption = false
	bindTimeoutSeconds   = 600
)

func TestCreate(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	defer close(stopCh)
	factory := newConfigFactory(client, v1.DefaultHardPodAffinitySymmetricWeight, stopCh)
	factory.Create()
}

// Test configures a scheduler from a policies defined in a file
// It combines some configurable predicate/priorities with some pre-defined ones
func TestCreateFromConfig(t *testing.T) {
	var configData []byte
	var policy schedulerapi.Policy

	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	defer close(stopCh)
	factory := newConfigFactory(client, v1.DefaultHardPodAffinitySymmetricWeight, stopCh)

	// Pre-register some predicate and priority functions
	RegisterFitPredicate("PredicateOne", PredicateOne)
	RegisterFitPredicate("PredicateTwo", PredicateTwo)
	RegisterPriorityFunction("PriorityOne", PriorityOne, 1)
	RegisterPriorityFunction("PriorityTwo", PriorityTwo, 1)

	configData = []byte(`{
		"kind" : "Policy",
		"apiVersion" : "v1",
		"predicates" : [
			{"name" : "TestZoneAffinity", "argument" : {"serviceAffinity" : {"labels" : ["zone"]}}},
			{"name" : "TestRequireZone", "argument" : {"labelsPresence" : {"labels" : ["zone"], "presence" : true}}},
			{"name" : "PredicateOne"},
			{"name" : "PredicateTwo"}
		],
		"priorities" : [
			{"name" : "RackSpread", "weight" : 3, "argument" : {"serviceAntiAffinity" : {"label" : "rack"}}},
			{"name" : "PriorityOne", "weight" : 2},
			{"name" : "PriorityTwo", "weight" : 1}		]
	}`)
	if err := runtime.DecodeInto(latestschedulerapi.Codec, configData, &policy); err != nil {
		t.Errorf("Invalid configuration: %v", err)
	}

	factory.CreateFromConfig(policy)
	hpa := factory.GetHardPodAffinitySymmetricWeight()
	if hpa != v1.DefaultHardPodAffinitySymmetricWeight {
		t.Errorf("Wrong hardPodAffinitySymmetricWeight, ecpected: %d, got: %d", v1.DefaultHardPodAffinitySymmetricWeight, hpa)
	}
}

func TestCreateFromConfigWithHardPodAffinitySymmetricWeight(t *testing.T) {
	var configData []byte
	var policy schedulerapi.Policy

	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	defer close(stopCh)
	factory := newConfigFactory(client, v1.DefaultHardPodAffinitySymmetricWeight, stopCh)

	// Pre-register some predicate and priority functions
	RegisterFitPredicate("PredicateOne", PredicateOne)
	RegisterFitPredicate("PredicateTwo", PredicateTwo)
	RegisterPriorityFunction("PriorityOne", PriorityOne, 1)
	RegisterPriorityFunction("PriorityTwo", PriorityTwo, 1)

	configData = []byte(`{
		"kind" : "Policy",
		"apiVersion" : "v1",
		"predicates" : [
			{"name" : "TestZoneAffinity", "argument" : {"serviceAffinity" : {"labels" : ["zone"]}}},
			{"name" : "TestRequireZone", "argument" : {"labelsPresence" : {"labels" : ["zone"], "presence" : true}}},
			{"name" : "PredicateOne"},
			{"name" : "PredicateTwo"}
		],
		"priorities" : [
			{"name" : "RackSpread", "weight" : 3, "argument" : {"serviceAntiAffinity" : {"label" : "rack"}}},
			{"name" : "PriorityOne", "weight" : 2},
			{"name" : "PriorityTwo", "weight" : 1}
		],
		"hardPodAffinitySymmetricWeight" : 10
	}`)
	if err := runtime.DecodeInto(latestschedulerapi.Codec, configData, &policy); err != nil {
		t.Errorf("Invalid configuration: %v", err)
	}
	factory.CreateFromConfig(policy)
	hpa := factory.GetHardPodAffinitySymmetricWeight()
	if hpa != 10 {
		t.Errorf("Wrong hardPodAffinitySymmetricWeight, ecpected: %d, got: %d", 10, hpa)
	}
}

func TestCreateFromEmptyConfig(t *testing.T) {
	var configData []byte
	var policy schedulerapi.Policy

	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	defer close(stopCh)
	factory := newConfigFactory(client, v1.DefaultHardPodAffinitySymmetricWeight, stopCh)

	configData = []byte(`{}`)
	if err := runtime.DecodeInto(latestschedulerapi.Codec, configData, &policy); err != nil {
		t.Errorf("Invalid configuration: %v", err)
	}

	factory.CreateFromConfig(policy)
}

// Test configures a scheduler from a policy that does not specify any
// predicate/priority.
// The predicate/priority from DefaultProvider will be used.
func TestCreateFromConfigWithUnspecifiedPredicatesOrPriorities(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	defer close(stopCh)
	factory := newConfigFactory(client, v1.DefaultHardPodAffinitySymmetricWeight, stopCh)

	RegisterFitPredicate("PredicateOne", PredicateOne)
	RegisterPriorityFunction("PriorityOne", PriorityOne, 1)

	RegisterAlgorithmProvider(DefaultProvider, sets.NewString("PredicateOne"), sets.NewString("PriorityOne"))

	configData := []byte(`{
		"kind" : "Policy",
		"apiVersion" : "v1"
	}`)
	var policy schedulerapi.Policy
	if err := runtime.DecodeInto(latestschedulerapi.Codec, configData, &policy); err != nil {
		t.Fatalf("Invalid configuration: %v", err)
	}

	config, err := factory.CreateFromConfig(policy)
	if err != nil {
		t.Fatalf("Failed to create scheduler from configuration: %v", err)
	}
	if _, found := config.Algorithm.Predicates()["PredicateOne"]; !found {
		t.Errorf("Expected predicate PredicateOne from %q", DefaultProvider)
	}
	if len(config.Algorithm.Prioritizers()) != 1 || config.Algorithm.Prioritizers()[0].Name != "PriorityOne" {
		t.Errorf("Expected priority PriorityOne from %q", DefaultProvider)
	}
}

// Test configures a scheduler from a policy that contains empty
// predicate/priority.
// Empty predicate/priority sets will be used.
func TestCreateFromConfigWithEmptyPredicatesOrPriorities(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	defer close(stopCh)
	factory := newConfigFactory(client, v1.DefaultHardPodAffinitySymmetricWeight, stopCh)

	RegisterFitPredicate("PredicateOne", PredicateOne)
	RegisterPriorityFunction("PriorityOne", PriorityOne, 1)

	RegisterAlgorithmProvider(DefaultProvider, sets.NewString("PredicateOne"), sets.NewString("PriorityOne"))

	configData := []byte(`{
		"kind" : "Policy",
		"apiVersion" : "v1",
		"predicates" : [],
		"priorities" : []
	}`)
	var policy schedulerapi.Policy
	if err := runtime.DecodeInto(latestschedulerapi.Codec, configData, &policy); err != nil {
		t.Fatalf("Invalid configuration: %v", err)
	}

	config, err := factory.CreateFromConfig(policy)
	if err != nil {
		t.Fatalf("Failed to create scheduler from configuration: %v", err)
	}
	if len(config.Algorithm.Predicates()) != 0 {
		t.Error("Expected empty predicate sets")
	}
	if len(config.Algorithm.Prioritizers()) != 0 {
		t.Error("Expected empty priority sets")
	}
}

func PredicateOne(pod *v1.Pod, meta predicates.PredicateMetadata, nodeInfo *schedulernodeinfo.NodeInfo) (bool, []predicates.PredicateFailureReason, error) {
	return true, nil, nil
}

func PredicateTwo(pod *v1.Pod, meta predicates.PredicateMetadata, nodeInfo *schedulernodeinfo.NodeInfo) (bool, []predicates.PredicateFailureReason, error) {
	return true, nil, nil
}

func PriorityOne(pod *v1.Pod, nodeNameToInfo map[string]*schedulernodeinfo.NodeInfo, nodes []*v1.Node) (schedulerapi.HostPriorityList, error) {
	return []schedulerapi.HostPriority{}, nil
}

func PriorityTwo(pod *v1.Pod, nodeNameToInfo map[string]*schedulernodeinfo.NodeInfo, nodes []*v1.Node) (schedulerapi.HostPriorityList, error) {
	return []schedulerapi.HostPriority{}, nil
}

func TestDefaultErrorFunc(t *testing.T) {
	testPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "bar"},
		Spec:       apitesting.V1DeepEqualSafePodSpec(),
	}
	client := fake.NewSimpleClientset(&v1.PodList{Items: []v1.Pod{*testPod}})
	stopCh := make(chan struct{})
	defer close(stopCh)
	queue := &internalqueue.FIFO{FIFO: cache.NewFIFO(cache.MetaNamespaceKeyFunc)}
	schedulerCache := schedulerinternalcache.New(30*time.Second, stopCh)
	podBackoff := util.CreatePodBackoff(1*time.Millisecond, 1*time.Second)
	errFunc := MakeDefaultErrorFunc(client, podBackoff, queue, schedulerCache, stopCh)

	errFunc(testPod, nil)

	for {
		// This is a terrible way to do this but I plan on replacing this
		// whole error handling system in the future. The test will time
		// out if something doesn't work.
		time.Sleep(10 * time.Millisecond)
		got, exists, _ := queue.Get(testPod)
		if !exists {
			continue
		}
		requestReceived := false
		actions := client.Actions()
		for _, a := range actions {
			if a.GetVerb() == "get" {
				getAction, ok := a.(clienttesting.GetAction)
				if !ok {
					t.Errorf("Can't cast action object to GetAction interface")
					break
				}
				name := getAction.GetName()
				ns := a.GetNamespace()
				if name != "foo" || ns != "bar" {
					t.Errorf("Expected name %s namespace %s, got %s %s",
						"foo", "bar", name, ns)
				}
				requestReceived = true
			}
		}
		if !requestReceived {
			t.Errorf("Get pod request not received")
		}
		if e, a := testPod, got; !reflect.DeepEqual(e, a) {
			t.Errorf("Expected %v, got %v", e, a)
		}
		break
	}
}

func TestNodeEnumerator(t *testing.T) {
	testList := &v1.NodeList{
		Items: []v1.Node{
			{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "bar"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "baz"}},
		},
	}
	me := nodeEnumerator{testList}

	if e, a := 3, me.Len(); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}
	for i := range testList.Items {
		t.Run(fmt.Sprintf("node enumerator/%v", i), func(t *testing.T) {
			gotObj := me.Get(i)
			if e, a := testList.Items[i].Name, gotObj.(*v1.Node).Name; e != a {
				t.Errorf("Expected %v, got %v", e, a)
			}
			if e, a := &testList.Items[i], gotObj; !reflect.DeepEqual(e, a) {
				t.Errorf("Expected %#v, got %v#", e, a)
			}
		})
	}
}

func TestBind(t *testing.T) {
	table := []struct {
		name    string
		binding *v1.Binding
	}{
		{
			name: "binding can bind and validate request",
			binding: &v1.Binding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: metav1.NamespaceDefault,
					Name:      "foo",
				},
				Target: v1.ObjectReference{
					Name: "foohost.kubernetes.mydomain.com",
				},
			},
		},
	}

	for _, test := range table {
		t.Run(test.name, func(t *testing.T) {
			testBind(test.binding, t)
		})
	}
}

func testBind(binding *v1.Binding, t *testing.T) {
	testPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: binding.GetName(), Namespace: metav1.NamespaceDefault},
		Spec:       apitesting.V1DeepEqualSafePodSpec(),
	}
	client := fake.NewSimpleClientset(&v1.PodList{Items: []v1.Pod{*testPod}})

	b := binder{client}

	if err := b.Bind(binding); err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	pod := client.CoreV1().Pods(metav1.NamespaceDefault).(*fakeV1.FakePods)

	actualBinding, err := pod.GetBinding(binding.GetName())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
		return
	}
	if !reflect.DeepEqual(binding, actualBinding) {
		t.Errorf("Binding did not match expectation")
		t.Logf("Expected: %v", binding)
		t.Logf("Actual:   %v", actualBinding)
	}
}

func TestInvalidHardPodAffinitySymmetricWeight(t *testing.T) {
	client := fake.NewSimpleClientset()
	// factory of "default-scheduler"
	stopCh := make(chan struct{})
	factory := newConfigFactory(client, -1, stopCh)
	defer close(stopCh)
	_, err := factory.Create()
	if err == nil {
		t.Errorf("expected err: invalid hardPodAffinitySymmetricWeight, got nothing")
	}
}

func TestInvalidFactoryArgs(t *testing.T) {
	client := fake.NewSimpleClientset()

	testCases := []struct {
		name                           string
		hardPodAffinitySymmetricWeight int32
		expectErr                      string
	}{
		{
			name:                           "symmetric weight below range",
			hardPodAffinitySymmetricWeight: -1,
			expectErr:                      "invalid hardPodAffinitySymmetricWeight: -1, must be in the range 0-100",
		},
		{
			name:                           "symmetric weight above range",
			hardPodAffinitySymmetricWeight: 101,
			expectErr:                      "invalid hardPodAffinitySymmetricWeight: 101, must be in the range 0-100",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			stopCh := make(chan struct{})
			factory := newConfigFactory(client, test.hardPodAffinitySymmetricWeight, stopCh)
			defer close(stopCh)
			_, err := factory.Create()
			if err == nil {
				t.Errorf("expected err: %s, got nothing", test.expectErr)
			}
		})
	}

}

func newConfigFactory(client clientset.Interface, hardPodAffinitySymmetricWeight int32, stopCh <-chan struct{}) Configurator {
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	return NewConfigFactory(&ConfigFactoryArgs{
		v1.DefaultSchedulerName,
		client,
		informerFactory.Core().V1().Nodes(),
		informerFactory.Core().V1().Pods(),
		informerFactory.Core().V1().PersistentVolumes(),
		informerFactory.Core().V1().PersistentVolumeClaims(),
		informerFactory.Core().V1().ReplicationControllers(),
		informerFactory.Apps().V1().ReplicaSets(),
		informerFactory.Apps().V1().StatefulSets(),
		informerFactory.Core().V1().Services(),
		informerFactory.Policy().V1beta1().PodDisruptionBudgets(),
		informerFactory.Storage().V1().StorageClasses(),
		hardPodAffinitySymmetricWeight,
		disablePodPreemption,
		schedulerapi.DefaultPercentageOfNodesToScore,
		bindTimeoutSeconds,
		stopCh,
	})
}

type fakeExtender struct {
	isBinder          bool
	interestedPodName string
	ignorable         bool
}

func (f *fakeExtender) Name() string {
	return "fakeExtender"
}

func (f *fakeExtender) IsIgnorable() bool {
	return f.ignorable
}

func (f *fakeExtender) ProcessPreemption(
	pod *v1.Pod,
	nodeToVictims map[*v1.Node]*schedulerapi.Victims,
	nodeNameToInfo map[string]*schedulernodeinfo.NodeInfo,
) (map[*v1.Node]*schedulerapi.Victims, error) {
	return nil, nil
}

func (f *fakeExtender) SupportsPreemption() bool {
	return false
}

func (f *fakeExtender) Filter(
	pod *v1.Pod,
	nodes []*v1.Node,
	nodeNameToInfo map[string]*schedulernodeinfo.NodeInfo,
) (filteredNodes []*v1.Node, failedNodesMap schedulerapi.FailedNodesMap, err error) {
	return nil, nil, nil
}

func (f *fakeExtender) Prioritize(
	pod *v1.Pod,
	nodes []*v1.Node,
) (hostPriorities *schedulerapi.HostPriorityList, weight int, err error) {
	return nil, 0, nil
}

func (f *fakeExtender) Bind(binding *v1.Binding) error {
	if f.isBinder {
		return nil
	}
	return errors.New("not a binder")
}

func (f *fakeExtender) IsBinder() bool {
	return f.isBinder
}

func (f *fakeExtender) IsInterested(pod *v1.Pod) bool {
	return pod != nil && pod.Name == f.interestedPodName
}

func TestGetBinderFunc(t *testing.T) {
	table := []struct {
		podName            string
		extenders          []algorithm.SchedulerExtender
		expectedBinderType string
		name               string
	}{
		{
			name:    "the extender is not a binder",
			podName: "pod0",
			extenders: []algorithm.SchedulerExtender{
				&fakeExtender{isBinder: false, interestedPodName: "pod0"},
			},
			expectedBinderType: "*factory.binder",
		},
		{
			name:    "one of the extenders is a binder and interested in pod",
			podName: "pod0",
			extenders: []algorithm.SchedulerExtender{
				&fakeExtender{isBinder: false, interestedPodName: "pod0"},
				&fakeExtender{isBinder: true, interestedPodName: "pod0"},
			},
			expectedBinderType: "*factory.fakeExtender",
		},
		{
			name:    "one of the extenders is a binder, but not interested in pod",
			podName: "pod1",
			extenders: []algorithm.SchedulerExtender{
				&fakeExtender{isBinder: false, interestedPodName: "pod1"},
				&fakeExtender{isBinder: true, interestedPodName: "pod0"},
			},
			expectedBinderType: "*factory.binder",
		},
	}

	for _, test := range table {
		t.Run(test.name, func(t *testing.T) {
			testGetBinderFunc(test.expectedBinderType, test.podName, test.extenders, t)
		})
	}
}

func testGetBinderFunc(expectedBinderType, podName string, extenders []algorithm.SchedulerExtender, t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
	}

	f := &configFactory{}
	binderFunc := getBinderFunc(f.client, extenders)
	binder := binderFunc(pod)

	binderType := fmt.Sprintf("%s", reflect.TypeOf(binder))
	if binderType != expectedBinderType {
		t.Errorf("Expected binder %q but got %q", expectedBinderType, binderType)
	}
}
