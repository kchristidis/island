// Code generated by counterfeiter. DO NOT EDIT.
package markendfakes

import (
	sync "sync"

	markend "github.com/kchristidis/island/markend"
)

type FakeInvoker struct {
	InvokeStub        func(string, int, string, []byte) ([]byte, error)
	invokeMutex       sync.RWMutex
	invokeArgsForCall []struct {
		arg1 string
		arg2 int
		arg3 string
		arg4 []byte
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

func (fake *FakeInvoker) Invoke(arg1 string, arg2 int, arg3 string, arg4 []byte) ([]byte, error) {
	var arg4Copy []byte
	if arg4 != nil {
		arg4Copy = make([]byte, len(arg4))
		copy(arg4Copy, arg4)
	}
	fake.invokeMutex.Lock()
	ret, specificReturn := fake.invokeReturnsOnCall[len(fake.invokeArgsForCall)]
	fake.invokeArgsForCall = append(fake.invokeArgsForCall, struct {
		arg1 string
		arg2 int
		arg3 string
		arg4 []byte
	}{arg1, arg2, arg3, arg4Copy})
	fake.recordInvocation("Invoke", []interface{}{arg1, arg2, arg3, arg4Copy})
	fake.invokeMutex.Unlock()
	if fake.InvokeStub != nil {
		return fake.InvokeStub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	fakeReturns := fake.invokeReturns
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeInvoker) InvokeCallCount() int {
	fake.invokeMutex.RLock()
	defer fake.invokeMutex.RUnlock()
	return len(fake.invokeArgsForCall)
}

func (fake *FakeInvoker) InvokeCalls(stub func(string, int, string, []byte) ([]byte, error)) {
	fake.invokeMutex.Lock()
	defer fake.invokeMutex.Unlock()
	fake.InvokeStub = stub
}

func (fake *FakeInvoker) InvokeArgsForCall(i int) (string, int, string, []byte) {
	fake.invokeMutex.RLock()
	defer fake.invokeMutex.RUnlock()
	argsForCall := fake.invokeArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeInvoker) InvokeReturns(result1 []byte, result2 error) {
	fake.invokeMutex.Lock()
	defer fake.invokeMutex.Unlock()
	fake.InvokeStub = nil
	fake.invokeReturns = struct {
		result1 []byte
		result2 error
	}{result1, result2}
}

func (fake *FakeInvoker) InvokeReturnsOnCall(i int, result1 []byte, result2 error) {
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

func (fake *FakeInvoker) Invocations() map[string][][]interface{} {
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

func (fake *FakeInvoker) recordInvocation(key string, args []interface{}) {
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

var _ markend.Invoker = new(FakeInvoker)
