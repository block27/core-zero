package keys

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/gob"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"github.com/amanelis/bespin/config"
	"github.com/amanelis/bespin/helpers"
	// "github.com/amanelis/bespin/services/bbolt"

	guuid "github.com/google/uuid"
)

type KeyAPI interface {
	Struct() *key

	Marshall() (string, error)
	Unmarshall(string) (KeyAPI, error)
}

// key - struct, main type and placeholder for private keys on the system. These
// should be persisted to a flat file database storage.
type key struct {
	sink sync.Mutex // mutex to allow clean concurrent access
	GID  guuid.UUID // guuid for crypto identification

	Fingerprint string

	PublicKeyPath  string
	PrivateKeyPath string
	PrivatePemPath string

	PublicKeyB64  string
	PrivateKeyB64 string
}

// NewECDSA - main factory method for creating the ECDSA key
func NewECDSA(c config.ConfigReader) (KeyAPI, error) {
	key := &key{
		GID: generateUUID(),
	}

	// Real key generation, need to eventually pipe in the rand.Reader
	// generated from PRNG and hardware devices
	pri, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	pub := &pri.PublicKey

	// PEM #1 - encoding
	pemKey, pemPub := encode(pri, pub)

	key.PublicKeyB64 = base64.StdEncoding.EncodeToString([]byte(pemPub))
	key.PrivateKeyB64 = base64.StdEncoding.EncodeToString([]byte(pemKey))
	key.Fingerprint = fmt.Sprintf("%s%s",
		pub.X.String()[0:12],
		pub.Y.String()[0:12],
	)

	// Create file paths which include the public keys curve as signature
	kDirPath := fmt.Sprintf("%s/%s", c.GetString("paths.keys"), key.GID.String())
	if _, err := os.Stat(kDirPath); os.IsNotExist(err) {
		os.Mkdir(kDirPath, os.ModePerm)
	}

	key.PrivateKeyPath = fmt.Sprintf("%s/%s", kDirPath, "private.key")
	key.PublicKeyPath = fmt.Sprintf("%s/%s", kDirPath, "public.key")
	key.PrivatePemPath = fmt.Sprintf("%s/%s", kDirPath, "private.pem")

	// save private and public key separately
	privatekeyFile, err := os.Create(key.PrivateKeyPath)
	if err != nil {
		return nil, err
	} else {
		privatekeyencoder := gob.NewEncoder(privatekeyFile)
		privatekeyencoder.Encode(pri)
		privatekeyFile.Close()
	}

	publickeyFile, err := os.Create(key.PublicKeyPath)
	if err != nil {
		return nil, err
	} else {
		publickeyencoder := gob.NewEncoder(publickeyFile)
		publickeyencoder.Encode(pub)
		publickeyFile.Close()
	}

	// Pem for private key
	pemfile, err := os.Create(key.PrivatePemPath)
	if err != nil {
		return nil, err
	}

	// Marshall the private key to PKCS8
	pem509, pemErr := x509.MarshalPKCS8PrivateKey(pri)
	if pemErr != nil {
		return nil, pemErr
	}

	pemkey := &pem.Block{
		Type:  "ECDSA PRIVATE KEY",
		Bytes: pem509,
	}

	// Create pem file
	e := pem.Encode(pemfile, pemkey)
	if e != nil {
		return nil, e
	}

	// Marshall the objects
	obj, err := keyToGOB64(key)
	if err != nil {
		return nil, err
	}

	// Write data to  file
	binFile := fmt.Sprintf("%s/%s", kDirPath, "obj.bin")
	objFile, err := os.Create(binFile)
	if err != nil {
		return nil, err
	}
	defer objFile.Close()

	if err := ioutil.WriteFile(binFile, []byte(obj), 0777); err != nil {
		return nil, err
	}

	// Save the data in Database

	return key, nil
}

// GetECDSA - return all system keys
func GetECDSA(c config.ConfigReader, fp string) (KeyAPI, error) {
	kDirPath := fmt.Sprintf("%s/%s", c.GetString("paths.keys"), fp)
	if _, err := os.Stat(kDirPath); os.IsNotExist(err) {
		return (*key)(nil), err
	}

	data, err := helpers.ReadFile(fmt.Sprintf("%s/obj.bin", kDirPath))
	if err != nil {
		return (*key)(nil), err
	}

	obj, err := keyFromGOB64(data)
	if err != nil {
		return (*key)(nil), err
	}

	return obj, nil
}

// Struct - return the full object for access to non exported fields
func (k *key) Struct() *key {
	return k
}

func (k *key) Marshall() (string, error) {
	d, err := keyToGOB64(k)
	if err != nil {
		return "", err
	}

	return d, nil
}

func (k *key) Unmarshall(obj string) (KeyAPI, error) {
	d, err := keyFromGOB64(obj)
	if err != nil {
		return (KeyAPI)(nil), err
	}

	return d, nil
}

// generateUUID - generate and return a valid GUUID
func generateUUID() guuid.UUID {
	return guuid.New()
}

func importPublicKeyfromPEM(pempub []byte) *ecdsa.PublicKey {
	block, _ := pem.Decode(pempub)
	pubInterface, _ := x509.ParsePKIXPublicKey(block.Bytes)
	pub := pubInterface.(*ecdsa.PublicKey)
	return pub
}

// export public key to pem format
func exportPublicKeytoPEM(pub *ecdsa.PublicKey) []byte {
	b, _ := x509.MarshalPKIXPublicKey(pub)
	c := pem.Block{
		Type:    "EC PUBLIC KEY",
		Headers: nil,
		Bytes:   b,
	}

	return pem.EncodeToMemory(&c)
}

// import private key from pem format
func importPrivateKeyfromPEM(pemsec []byte) *ecdsa.PrivateKey {
	block, _ := pem.Decode(pemsec)
	sec, _ := x509.ParseECPrivateKey(block.Bytes)
	return sec
}

// export private key to pem format
func exportPrivateKeytoPEM(sec *ecdsa.PrivateKey) []byte {
	l, _ := x509.MarshalECPrivateKey(sec)
	m := pem.Block{
		Type:    "EC PRIVATE KEY",
		Headers: nil,
		Bytes:   l,
	}
	n := pem.EncodeToMemory(&m)

	keypem, _ := os.OpenFile("sec.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	pem.Encode(keypem, &pem.Block{Type: "EC PRIVATE KEY", Bytes: l})

	return n
}

// import private key from pem format
func importPrivateKeyfromEncryptedPEM(pemsec, password []byte) *ecdsa.PrivateKey {
	block, _ := pem.Decode(pemsec)
	buf, _ := x509.DecryptPEMBlock(block, password)
	sec, _ := x509.ParseECPrivateKey(buf)
	return sec
}

// export private key to pem format
func exportPrivateKeytoEncryptedPEM(sec *ecdsa.PrivateKey, password []byte) []byte {
	l, _ := x509.MarshalECPrivateKey(sec)
	m, _ := x509.EncryptPEMBlock(rand.Reader, "EC PRIVATE KEY", l, password, x509.PEMCipherAES256)
	n := pem.EncodeToMemory(m)

	keypem, _ := os.OpenFile("sec.Encrypted.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	pem.Encode(keypem, &pem.Block{Type: "EC PRIVATE KEY", Bytes: l})

	return n
}

func encode(privateKey *ecdsa.PrivateKey, publicKey *ecdsa.PublicKey) (string, string) {
	x509Encoded, _ := x509.MarshalECPrivateKey(privateKey)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})

	x509EncodedPub, _ := x509.MarshalPKIXPublicKey(publicKey)
	pemEncodedPub := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509EncodedPub})

	return string(pemEncoded), string(pemEncodedPub)
}

func decode(pemEncoded string, pemEncodedPub string) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	block, _ := pem.Decode([]byte(pemEncoded))
	x509Encoded := block.Bytes
	privateKey, _ := x509.ParseECPrivateKey(x509Encoded)

	blockPub, _ := pem.Decode([]byte(pemEncodedPub))
	x509EncodedPub := blockPub.Bytes
	genericPublicKey, _ := x509.ParsePKIXPublicKey(x509EncodedPub)
	publicKey := genericPublicKey.(*ecdsa.PublicKey)

	return privateKey, publicKey
}

func keyToGOB64(k *key) (string, error) {
	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)

	if err := e.Encode(k); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b.Bytes()), nil
}

func keyFromGOB64(str string) (*key, error) {
	by, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return (*key)(nil), err
	}

	b := bytes.Buffer{}
	b.Write(by)
	d := gob.NewDecoder(&b)

	var k *key

	if err = d.Decode(&k); err != nil {
		fmt.Println(`failed gob Decode`, err)
	}

	return k, nil
}
