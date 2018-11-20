package main

import (
	"crypto/rsa"
	"sync"
)

// Constants ...
const (
	TraceLength  = 35036
	BufferLen    = 20
	EnableEvents = false
)

type aggregateStats struct {
	LateTXsCount, LateBuysCount, LateSellsCount [TraceLength]int
	// lateDecryptsCount [TraceLength]int
	// duplTXsCount, duplBuysCount, duplSellsCount [TraceLength]int
}

var (
	buyerBids, sellerBids map[int]BidCollection
	bbMutex, sbMutex      sync.RWMutex
	aggStats              aggregateStats
)

// Keys
var keyPair *rsa.PrivateKey
var privBytes = []byte(`
-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEAyqvL/UJzq7uSETf+I8e+OMjams1ZYMvXF8PjzBhZJzrgJNTi
zJDAkFrnAPFTstflg8kU7wxkNOmH4steDIQ9hi+xAW8JRELKinIriQZJ+lSp6/ds
7Kjb0EfPzDF/nbJYjtdIVHIPfaLZno5O46CsmRt2KiwYdKDf08iHBzifugV8mtq2
ugxczA3PvgQ89Yyubcccprg/Zy54lOziy0Pn9mDePOI4oIOM0XO4VGbfbVZtAC9H
Lv4jiO4nB+ixem+3jGNR7IwsB+0xlcN1zvBic1WBNlJYqiEaIZTzqDHi8KdigQS7
bO0nMttS7Wr0WWyXHSPtrRmKvx04CY+IPHkWcQIDAQABAoIBABHvslXvk50XNI4h
jnRMMSGFZRNeKRLP93E6/OYLIZi/NScNUCUainA8G0WSFf417TIEkb22MwgbwtLn
fKNO8ML3ZYri8McBwjsOb5vo2pM0+vTPKOyo5QtBz7oah1jFd+DsXJJcpdJQn0HR
BlpO1feW3pZM4L0xn512mbyh3kDwIwzhzxnHzXnhD78bzrm7JoBBITcrjt228Z+3
mVNCftc48SGoVMnOMJr8B6tlOkL0tG3A593hZZfyCxrdsRmWRieQJACWqhfXJFPh
JXM8qxjtqCTauUKc3NEuWsqsmObN6Cpq8L1K+/bmjaNl6qeGigLHpWyN5ks/U9FP
7VDiECECgYEA5pWiGyQWYHYPntBpqlmfhb8zKUfBZahFnzFo9CKV7YDK7cUx3vcM
I5Q94btAQg6kD54dQMWzudmUb3XVU5+ZJ3yBgUTFqfSvcPopbrtrSji3u0pRUyRg
UgMfUN0orPLlAWLTlpy4SDsfcKeQHSfS+L8Eu0Kt+AJErs0I5eCIatsCgYEA4QKI
8IZ5BRbaGlJvPI98J9NRJjRhkxGh55JslNguD+w88Q8bDltMlEQRWfpkKHwMyVMB
Ll2VHg/Dg3KdQUkItv+27qMRP2eUFCj0XQo+5owmYHhpn5fYHJjYrsDSrNZDqSL/
kel3MuxSz9NK3EAGToIQtfAZhwjeOhzvNqB4N6MCgYEA1EkOZU5kC4ql9uCJZ3v7
kXbl8ytMsfqpnlYu+hSdU3svWJgjwdJQKrFgB2INVsOD55z58ZgSTxgxwCwLqmFU
7zWBRTG7iSzsGGc3neqObFarUJKrLJBg3SBixF/YAuHcU9pYUmEWh+lmmKCr3Su8
36V9Bant4Fa2RPgfKQP+k+ECgYEAk4ev9dSVoMqc8kk+efyyMQKS4HPTzjPvbgBJ
hUZA3VvNkViQKted3FDM96v+47SCRbZQve/KB83aKWOKy/Vw61u6u7jbZDErnBRG
NIK1P0CBIRuSVXufzRBCckInX/+UmV9DJo5nA1KD8ZPeL48jE3KgNkpY0nr0CjJS
fgS1DfUCgYEAmx3sAJqR/mksm8jQXiduGe2L9IwYoWvvVL6GzMik58iuI2OtmjF8
OSwjLClaBdvj45YrUN3/i5LiIvblup9QpqshyEbSuT5gvhuf3jVmb08TJszihVXJ
p2j5extRQ8Ua4T37+7UdkZMvJLEps9ic+IlJWYwKt1qM+ZIZuuitmwU=
-----END RSA PRIVATE KEY-----`)