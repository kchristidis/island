// Code generated by counterfeiter. DO NOT EDIT.
package blockfakes

import (
	sync "sync"

	ledger "github.com/hyperledger/fabric-sdk-go/pkg/client/ledger"
	fab "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	block "github.com/kchristidis/exp2/block"
)

type FakeLedgerQuerier struct {
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

func (fake *FakeLedgerQuerier) QueryInfo(arg1 ...ledger.RequestOption) (*fab.BlockchainInfoResponse, error) {
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

func (fake *FakeLedgerQuerier) QueryInfoCallCount() int {
	fake.queryInfoMutex.RLock()
	defer fake.queryInfoMutex.RUnlock()
	return len(fake.queryInfoArgsForCall)
}

func (fake *FakeLedgerQuerier) QueryInfoCalls(stub func(...ledger.RequestOption) (*fab.BlockchainInfoResponse, error)) {
	fake.queryInfoMutex.Lock()
	defer fake.queryInfoMutex.Unlock()
	fake.QueryInfoStub = stub
}

func (fake *FakeLedgerQuerier) QueryInfoArgsForCall(i int) []ledger.RequestOption {
	fake.queryInfoMutex.RLock()
	defer fake.queryInfoMutex.RUnlock()
	argsForCall := fake.queryInfoArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeLedgerQuerier) QueryInfoReturns(result1 *fab.BlockchainInfoResponse, result2 error) {
	fake.queryInfoMutex.Lock()
	defer fake.queryInfoMutex.Unlock()
	fake.QueryInfoStub = nil
	fake.queryInfoReturns = struct {
		result1 *fab.BlockchainInfoResponse
		result2 error
	}{result1, result2}
}

func (fake *FakeLedgerQuerier) QueryInfoReturnsOnCall(i int, result1 *fab.BlockchainInfoResponse, result2 error) {
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

func (fake *FakeLedgerQuerier) Invocations() map[string][][]interface{} {
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

func (fake *FakeLedgerQuerier) recordInvocation(key string, args []interface{}) {
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

var _ block.LedgerQuerier = new(FakeLedgerQuerier)