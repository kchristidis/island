// Code generated by counterfeiter. DO NOT EDIT.
package blocknotifierfakes

import (
	sync "sync"

	ledger "github.com/hyperledger/fabric-sdk-go/pkg/client/ledger"
	fab "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	blocknotifier "github.com/kchristidis/exp2/blocknotifier"
)

type FakeQuerier struct {
	QueryInfoStub        func(...ledger.RequestOption) (*fab.BlockchainInfoResponse, error)
	queryInfoMutex       sync.RWMutex
	queryInfoArgsForCall []struct {
		arg1 []ledger.RequestOption
	}
	queryInfoReturns struct {
		result1 *fab.BlockchainInfoResponse
		result2 error
	}
	queryInfoReturnsOnCall map[int]struct {
		result1 *fab.BlockchainInfoResponse
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeQuerier) QueryInfo(arg1 ...ledger.RequestOption) (*fab.BlockchainInfoResponse, error) {
	fake.queryInfoMutex.Lock()
	ret, specificReturn := fake.queryInfoReturnsOnCall[len(fake.queryInfoArgsForCall)]
	fake.queryInfoArgsForCall = append(fake.queryInfoArgsForCall, struct {
		arg1 []ledger.RequestOption
	}{arg1})
	fake.recordInvocation("QueryInfo", []interface{}{arg1})
	fake.queryInfoMutex.Unlock()
	if fake.QueryInfoStub != nil {
		return fake.QueryInfoStub(arg1...)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	fakeReturns := fake.queryInfoReturns
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeQuerier) QueryInfoCallCount() int {
	fake.queryInfoMutex.RLock()
	defer fake.queryInfoMutex.RUnlock()
	return len(fake.queryInfoArgsForCall)
}

func (fake *FakeQuerier) QueryInfoCalls(stub func(...ledger.RequestOption) (*fab.BlockchainInfoResponse, error)) {
	fake.queryInfoMutex.Lock()
	defer fake.queryInfoMutex.Unlock()
	fake.QueryInfoStub = stub
}

func (fake *FakeQuerier) QueryInfoArgsForCall(i int) []ledger.RequestOption {
	fake.queryInfoMutex.RLock()
	defer fake.queryInfoMutex.RUnlock()
	argsForCall := fake.queryInfoArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeQuerier) QueryInfoReturns(result1 *fab.BlockchainInfoResponse, result2 error) {
	fake.queryInfoMutex.Lock()
	defer fake.queryInfoMutex.Unlock()
	fake.QueryInfoStub = nil
	fake.queryInfoReturns = struct {
		result1 *fab.BlockchainInfoResponse
		result2 error
	}{result1, result2}
}

func (fake *FakeQuerier) QueryInfoReturnsOnCall(i int, result1 *fab.BlockchainInfoResponse, result2 error) {
	fake.queryInfoMutex.Lock()
	defer fake.queryInfoMutex.Unlock()
	fake.QueryInfoStub = nil
	if fake.queryInfoReturnsOnCall == nil {
		fake.queryInfoReturnsOnCall = make(map[int]struct {
			result1 *fab.BlockchainInfoResponse
			result2 error
		})
	}
	fake.queryInfoReturnsOnCall[i] = struct {
		result1 *fab.BlockchainInfoResponse
		result2 error
	}{result1, result2}
}

func (fake *FakeQuerier) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.queryInfoMutex.RLock()
	defer fake.queryInfoMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeQuerier) recordInvocation(key string, args []interface{}) {
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

var _ blocknotifier.Querier = new(FakeQuerier)