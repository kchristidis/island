// Code generated by counterfeiter. DO NOT EDIT.
package markerfakes

import (
	sync "sync"

	marker "github.com/kchristidis/exp2/marker"
)

type FakeSDKer struct {
	InvokeStub        func(int, string, []byte) ([]byte, error)
	invokeMutex       sync.RWMutex
	invokeArgsForCall []struct {
		arg1 int
		arg2 string
		arg3 []byte
	}
	invokeReturns struct {
		result1 []byte
		result2 error
	}
	invokeReturnsOnCall map[int]struct {
		result1 []byte
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeSDKer) Invoke(arg1 int, arg2 string, arg3 []byte) ([]byte, error) {
	var arg3Copy []byte
	if arg3 != nil {
		arg3Copy = make([]byte, len(arg3))
		copy(arg3Copy, arg3)
	}
	fake.invokeMutex.Lock()
	ret, specificReturn := fake.invokeReturnsOnCall[len(fake.invokeArgsForCall)]
	fake.invokeArgsForCall = append(fake.invokeArgsForCall, struct {
		arg1 int
		arg2 string
		arg3 []byte
	}{arg1, arg2, arg3Copy})
	fake.recordInvocation("Invoke", []interface{}{arg1, arg2, arg3Copy})
	fake.invokeMutex.Unlock()
	if fake.InvokeStub != nil {
		return fake.InvokeStub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	fakeReturns := fake.invokeReturns
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeSDKer) InvokeCallCount() int {
	fake.invokeMutex.RLock()
	defer fake.invokeMutex.RUnlock()
	return len(fake.invokeArgsForCall)
}

func (fake *FakeSDKer) InvokeCalls(stub func(int, string, []byte) ([]byte, error)) {
	fake.invokeMutex.Lock()
	defer fake.invokeMutex.Unlock()
	fake.InvokeStub = stub
}

func (fake *FakeSDKer) InvokeArgsForCall(i int) (int, string, []byte) {
	fake.invokeMutex.RLock()
	defer fake.invokeMutex.RUnlock()
	argsForCall := fake.invokeArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeSDKer) InvokeReturns(result1 []byte, result2 error) {
	fake.invokeMutex.Lock()
	defer fake.invokeMutex.Unlock()
	fake.InvokeStub = nil
	fake.invokeReturns = struct {
		result1 []byte
		result2 error
	}{result1, result2}
}

func (fake *FakeSDKer) InvokeReturnsOnCall(i int, result1 []byte, result2 error) {
	fake.invokeMutex.Lock()
	defer fake.invokeMutex.Unlock()
	fake.InvokeStub = nil
	if fake.invokeReturnsOnCall == nil {
		fake.invokeReturnsOnCall = make(map[int]struct {
			result1 []byte
			result2 error
		})
	}
	fake.invokeReturnsOnCall[i] = struct {
		result1 []byte
		result2 error
	}{result1, result2}
}

func (fake *FakeSDKer) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.invokeMutex.RLock()
	defer fake.invokeMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeSDKer) recordInvocation(key string, args []interface{}) {
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

var _ marker.SDKer = new(FakeSDKer)
