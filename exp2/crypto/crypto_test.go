package crypto_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/kchristidis/island/exp2/crypto"
	"github.com/stretchr/testify/require"
)

func TestEncrypt(t *testing.T) {
	input := []byte("foo")
	keyPair, _ := crypto.Generate()
	cipherText, _ := crypto.Encrypt(input, &(keyPair.PublicKey))
	plainText, _ := crypto.Decrypt(cipherText, keyPair)
	require.Equal(t, input, plainText, "Decrypted message not identical to input")
}

func TestSerialize(t *testing.T) {
	input := []byte("foo")
	privBytes := []byte(`
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
	pubBytes := []byte(`
-----BEGIN RSA PUBLIC KEY-----
MIIBCgKCAQEAyqvL/UJzq7uSETf+I8e+OMjams1ZYMvXF8PjzBhZJzrgJNTizJDA
kFrnAPFTstflg8kU7wxkNOmH4steDIQ9hi+xAW8JRELKinIriQZJ+lSp6/ds7Kjb
0EfPzDF/nbJYjtdIVHIPfaLZno5O46CsmRt2KiwYdKDf08iHBzifugV8mtq2ugxc
zA3PvgQ89Yyubcccprg/Zy54lOziy0Pn9mDePOI4oIOM0XO4VGbfbVZtAC9HLv4j
iO4nB+ixem+3jGNR7IwsB+0xlcN1zvBic1WBNlJYqiEaIZTzqDHi8KdigQS7bO0n
MttS7Wr0WWyXHSPtrRmKvx04CY+IPHkWcQIDAQAB
-----END RSA PUBLIC KEY-----`)

	pubKey, err := crypto.DeserializePublic(pubBytes)
	require.Nil(t, err)
	cipherText, err := crypto.Encrypt(input, pubKey)
	require.Nil(t, err)

	keyPair, err := crypto.DeserializePrivate(privBytes)
	require.Nil(t, err)
	plainText, err := crypto.Decrypt(cipherText, keyPair)
	require.Nil(t, err)

	require.Equal(t, input, plainText, "Decrypted message not identical to input")
}

func TestPersist(t *testing.T) {
	dir, err := ioutil.TempDir("", "crypto-test")
	require.NoError(t, err, "Unable to create temp dir: %s", err)
	defer os.RemoveAll(dir)

	keyPair, _ := crypto.Generate()

	t.Run("Private", func(t *testing.T) {
		privName := filepath.Join(dir, "priv.pem")
		crypto.PersistPrivate(keyPair, privName)

		res, err := crypto.LoadPrivate(privName)
		require.NoError(t, err, "Cannot load key from disk: %s", err)
		require.Equal(t, keyPair, res, "Persisted key does not match loaded key")
	})

	t.Run("Public", func(t *testing.T) {
		pubName := filepath.Join(dir, "pub.pem")
		crypto.PersistPublic(&keyPair.PublicKey, pubName)

		res, err := crypto.LoadPublic(pubName)
		require.NoError(t, err, "Cannot load key from disk: %s", err)
		require.Equal(t, &keyPair.PublicKey, res, "Persisted key does not match loaded key")
	})
}
