package main

import (
	"reflect"
	"testing"
	"time"

	samplecrdv1 "github.com/tonyshanc/sample-operator-v1/pkg/apis/samplecrd/v1"
	"github.com/tonyshanc/sample-operator-v1/pkg/client/clientset/versioned/fake"
	informers "github.com/tonyshanc/sample-operator-v1/pkg/client/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

var (
	alwaysReady        = func() bool { return true }
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

type fixture struct {
	t          *testing.T
	client     *fake.Clientset
	kubeCLient *k8sfake.Clientset
	// Objects to put into the Indexer store. Local
	CarLister []*samplecrdv1.Car
	// Actions expected to happen on the client
	actions []core.Action
	// Objects from here preloaded into NewSimpleFake. Expected
	objects []runtime.Object
}

func newFixture(t *testing.T) *fixture {
	return &fixture{
		t:       t,
		objects: []runtime.Object{},
	}
}

func newCar(name string) *samplecrdv1.Car {
	return &samplecrdv1.Car{
		TypeMeta: metav1.TypeMeta{APIVersion: samplecrdv1.SchemeGroupVersion.Version},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: samplecrdv1.CarSpec{
			Status: "idle",
		},
	}
}

func (f *fixture) newController() (*Controller, informers.SharedInformerFactory) {
	f.client = fake.NewSimpleClientset(f.objects...)
	i := informers.NewSharedInformerFactory(f.client, noResyncPeriodFunc())
	c := NewController(f.kubeCLient, f.client, i.Samplecrd().V1().Cars())
	c.carsSynced = alwaysReady
	c.recorder = &record.FakeRecorder{}

	for _, f := range f.CarLister {
		i.Samplecrd().V1().Cars().Informer().GetIndexer().Add(f)
	}
	return c, i
}

func (f *fixture) run(carName string) {
	f.runController(carName, true, false)
}

func (f *fixture) runController(carName string, startinformers bool, expectError bool) {
	c, i := f.newController()
	if startinformers {
		stopCh := make(chan struct{})
		defer close(stopCh)
		i.Start(stopCh)
	}

	err := c.syncHandler(carName)
	if !expectError && err != nil {
		f.t.Errorf("expected error syncing %v", err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing car, got nil")
	}

	f.t.Log("f.Client Actions: ", f.client.Actions())
	actions := filterInformerActions(f.client.Actions())
	for i, action := range actions {
		if len(f.actions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(actions)-len(f.actions), actions[i:])
			break
		}

		expectedAction := f.actions[i]
		checkAction(expectedAction, action, f.t)
	}

	if len(f.actions) > len(actions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.actions)-len(actions), f.actions[len(actions):])
	}
}

func checkAction(expected, actual core.Action, t *testing.T) {
	if !(expected.Matches(actual.GetVerb(), actual.GetResource().Resource) && actual.GetSubresource() == expected.GetSubresource()) {
		t.Errorf("Expected\n\t%#v\ngot\n\t%#v", expected, actual)
		return
	}

	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("Action has wrong type. Expected: %t. Got: %t", expected, actual)
		return
	}

	switch a := actual.(type) {
	case core.CreateActionImpl:
		e, _ := expected.(core.CreateActionImpl)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expObject, object))
		}
	case core.UpdateActionImpl:
		e, _ := expected.(core.UpdateActionImpl)
		expObject := e.GetObject()
		object := a.GetObject()
		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expObject, object))
		}
	case core.PatchActionImpl:
		e, _ := expected.(core.PatchActionImpl)
		expPatch := e.GetPatch()
		patch := a.GetPatch()

		if !reflect.DeepEqual(expPatch, patch) {
			t.Errorf("Action %s %s has wrong patch\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expPatch, patch))
		}
	default:
		t.Errorf("Uncaptured Action %s %s, you should explicitly add a case to capture it",
			actual.GetVerb(), actual.GetResource().Resource)
	}
}

// filterInformerActions filters list and watch actions for testing resources.
// Since list and watch don't change resource state we can filter it to lower
// nose level in our tests.
func filterInformerActions(actions []core.Action) []core.Action {
	ret := []core.Action{}
	for _, action := range actions {
		if len(action.GetNamespace()) == 0 &&
			(action.Matches("list", "cars")) ||
			action.Matches("watch", "cars") {
			continue
		}
		ret = append(ret, action)
	}
	return ret
}

func getKey(car *samplecrdv1.Car, t *testing.T) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(car)
	if err != nil {
		t.Errorf("Unexpected error getting key for car %v: %v", car.Name, err)
		return ""
	}
	return key
}

func (f *fixture) expectCreateCarAction(car *samplecrdv1.Car) {
	action := core.NewCreateAction(schema.GroupVersionResource{Resource: "cars"}, car.Namespace, car)
	f.actions = append(f.actions, action)
}

func TestDoNothing(t *testing.T) {
	f := newFixture(t)
	car := newCar("test")
	f.CarLister = append(f.CarLister, car)
	f.objects = append(f.objects, car)
	f.expectCreateCarAction(car)
	f.run(getKey(car, t))
}

func TestCreateCar(t *testing.T) {
	f := newFixture(t)
	car := newCar("test")
	f.CarLister = append(f.CarLister, car)
	f.objects = append(f.objects, car)
	f.expectCreateCarAction(car)
	f.run(getKey(car, t))
}

func int32Ptr(i int32) *int32 { return &i }
