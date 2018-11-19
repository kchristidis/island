// Code generated by counterfeiter. DO NOT EDIT.
package markendfakes

import (
	sync "sync"

	markend "github.com/kchristidis/island/exp2/markend"
)

type FakeNotifier struct {
	RegisterStub        func(int, chan int) bool
	registerMutex       sync.RWMutex
	registerArgsForCall []struct {
		arg1 int
		arg2 chan int
	}
	registerReturns struct {
		result1 bool
	}
	registerReturnsOnCall map[int]struct {
		result1 bool
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeNotifier) Register(arg1 int, arg2 chan int) bool {
	fake.registerMutex.Lock()
	ret, specificReturn := fake.registerReturnsOnCall[len(fake.registerArgsForCall)]
	fake.registerArgsForCall = append(fake.registerArgsForCall, struct {
		arg1 int
		arg2 chan int
	}{arg1, arg2})
	fake.recordInvocation("Register", []interface{}{arg1, arg2})
	fake.registerMutex.Unlock()
	if fake.RegisterStub != nil {
		return fake.RegisterStub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1
	}
	fakeReturns := fake.registerReturns
	return fakeReturns.result1
}

func (fake *FakeNotifier) RegisterCallCount() int {
	fake.registerMutex.RLock()
	defer fake.registerMutex.RUnlock()
	return len(fake.registerArgsForCall)
}

func (fake *FakeNotifier) RegisterCalls(stub func(int, chan int) bool) {
	fake.registerMutex.Lock()
	defer fake.registerMutex.Unlock()
	fake.RegisterStub = stub
}

func (fake *FakeNotifier) RegisterArgsForCall(i int) (int, chan int) {
	fake.registerMutex.RLock()
	defer fake.registerMutex.RUnlock()
	argsForCall := fake.registerArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeNotifier) RegisterReturns(result1 bool) {
	fake.registerMutex.Lock()
	defer fake.registerMutex.Unlock()
	fake.RegisterStub = nil
	fake.registerReturns = struct {
		result1 bool
	}{result1}
}

func (fake *FakeNotifier) RegisterReturnsOnCall(i int, result1 bool) {
	fake.registerMutex.Lock()
	defer fake.registerMutex.Unlock()
	fake.RegisterStub = nil
	if fake.registerReturnsOnCall == nil {
		fake.registerReturnsOnCall = make(map[int]struct {
			result1 bool
		})
	}
	fake.registerReturnsOnCall[i] = struct {
		result1 bool
	}{result1}
}

func (fake *FakeNotifier) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.registerMutex.RLock()
	defer fake.registerMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeNotifier) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ markend.Notifier = new(FakeNotifier)