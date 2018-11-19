package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
)

// Filenames ...
const (
	Private = "priv.pem"
	Public  = "pub.pem"
)

// Generate generates a key pair.
// See: https://medium.com/@raul_11817/golang-cryptography-rsa-asymmetric-algorithm-e91363a2f7b3
func Generate() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// Encrypt encryptes a message using a given public key.
func Encrypt(msg []byte, pubKey *rsa.PublicKey) ([]byte, error) {
	return rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, msg, nil)
}

// Decrypt decrypts a message using a given private key.
func Decrypt(cipherText []byte, privKey *rsa.PrivateKey) ([]byte, error) {
	return rsa.DecryptOAEP(sha256.New(), rand.Reader, privKey, cipherText, nil)
}

// SerializePrivate returns the PEM encoding of a private key.
func SerializePrivate(privKey *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privKey),
		},
	)
}

// PersistPrivate persists the PEM encoding of a private key to disk.
// See: https://stackoverflow.com/a/13555138/2363529
// And: https://stackoverflow.com/a/25324848/2363529
func PersistPrivate(privKey *rsa.PrivateKey, filename string) error {
	return ioutil.WriteFile(filename, SerializePrivate(privKey), 0644)
}

// SerializePublic returns the PEM encoding of a public key.
func SerializePublic(pubKey *rsa.PublicKey) []byte {
	return pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: x509.MarshalPKCS1PublicKey(pubKey),
		},
	)
}

// DeserializePrivate decodes a PEM-encoded private key.
func DeserializePrivate(privBytes []byte) (*rsa.PrivateKey, error) {
	pemBlock, _ := pem.Decode(privBytes)
	if pemBlock == nil {
		return nil, fmt.Errorf("Failed to parse PEM block from given input")
	}

	return x509.ParsePKCS1PrivateKey(pemBlock.Bytes)
}

// DeserializePublic decodes a PEM-encoded public key.
func DeserializePublic(pubBytes []byte) (*rsa.PublicKey, error) {
	pemBlock, _ := pem.Decode(pubBytes)
	if pemBlock == nil {
		return nil, fmt.Errorf("Failed to parse PEM block from given input")
	}

	return x509.ParsePKCS1PublicKey(pemBlock.Bytes)
}

// PersistPublic persists the PEM encoding of a public key to disk.
func PersistPublic(pubKey *rsa.PublicKey, filename string) error {
	return ioutil.WriteFile(filename, SerializePublic(pubKey), 0644)
}

// LoadPrivate reads and decodes a PEM-encoded private key from disk.
// See: https://stackoverflow.com/a/44688503/2363529
func LoadPrivate(filename string) (*rsa.PrivateKey, error) {
	keyBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return DeserializePrivate(keyBytes)
}

// LoadPublic reads and decodes a PEM-encoded public key from disk.
func LoadPublic(filename string) (*rsa.PublicKey, error) {
	keyBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return DeserializePublic(keyBytes)
}
