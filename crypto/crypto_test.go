package crypto_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/kchristidis/exp2/crypto"
	"github.com/stretchr/testify/require"
)

func TestEncrypt(t *testing.T) {
	input := []byte("foo")
	keyPair, _ := crypto.Generate()
	cipherText, _ := crypto.Encrypt(input, &(keyPair.PublicKey))
	plainText, _ := crypto.Decrypt(cipherText, keyPair)
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
