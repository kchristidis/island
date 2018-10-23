// Code generated by counterfeiter. DO NOT EDIT.
package markerfakes

import (
	sync "sync"

	marker "github.com/kchristidis/exp2/marker"
)

type FakeSlotter struct {
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

func (fake *FakeSlotter) Register(arg1 int, arg2 chan int) bool {
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

func (fake *FakeSlotter) RegisterCallCount() int {
	fake.registerMutex.RLock()
	defer fake.registerMutex.RUnlock()
	return len(fake.registerArgsForCall)
}

func (fake *FakeSlotter) RegisterCalls(stub func(int, chan int) bool) {
	fake.registerMutex.Lock()
	defer fake.registerMutex.Unlock()
	fake.RegisterStub = stub
}

func (fake *FakeSlotter) RegisterArgsForCall(i int) (int, chan int) {
	fake.registerMutex.RLock()
	defer fake.registerMutex.RUnlock()
	argsForCall := fake.registerArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeSlotter) RegisterReturns(result1 bool) {
	fake.registerMutex.Lock()
	defer fake.registerMutex.Unlock()
	fake.RegisterStub = nil
	fake.registerReturns = struct {
		result1 bool
	}{result1}
}

func (fake *FakeSlotter) RegisterReturnsOnCall(i int, result1 bool) {
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

func (fake *FakeSlotter) Invocations() map[string][][]interface{} {
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

func (fake *FakeSlotter) recordInvocation(key string, args []interface{}) {
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

var _ marker.Slotter = new(FakeSlotter)
