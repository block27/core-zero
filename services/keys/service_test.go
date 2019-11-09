package keys

import (
	"crypto/ecdsa"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/amanelis/bespin/config"
	"github.com/amanelis/bespin/helpers"
)

var Config config.ConfigReader

var PrivateKey *ecdsa.PrivateKey
var PublicKey *ecdsa.PublicKey

var Key *key

func init() {
	os.Setenv("ENVIRONMENT", "test")

	c, err := config.LoadConfig(config.ConfigDefaults)
	if err != nil {
		panic(err)
	}

	if c.GetString("environment") != "test" {
		panic(fmt.Errorf("test [environment] is not in [test] mode"))
	}

	k1, err := NewECDSA(c, "test-key")
	if err != nil {
		panic(err)
	}

	Key = k1.Struct()
	Config = c
}

func TestNewECDSABlank(t *testing.T) {
	result, err := NewECDSABlank(Config)
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, result.Struct().GID.String(), "00000000-0000-0000-0000-000000000000")
	assert.Equal(t, result.Struct().Name, "")
	assert.Equal(t, result.Struct().Slug, "")
	assert.Equal(t, result.Struct().Status, "")
	assert.Equal(t, result.Struct().KeySize, 0)
	assert.Equal(t, result.Struct().FingerprintMD5, "")
	assert.Equal(t, result.Struct().FingerprintSHA, "")
}

func TestGetECDSA(t *testing.T) {
	result, err := GetECDSA(Config, Key.FilePointer())
	if err != nil {
		t.Fail()
	}

	assert.NotNil(t, result.Struct().GID)
	assert.NotNil(t, result.Struct().FingerprintMD5)
	assert.NotNil(t, result.Struct().FingerprintSHA)
}

func TestListECDSA(t *testing.T) {
	_, err := NewECDSA(Config, "context-key")
	if err != nil {
		t.Fail()
	}

	result, err := ListECDSA(Config)
	if err != nil {
		t.Fail()
	}

	if len(result) == 0 {
		t.Fail()
	}
}

func TestGenerateUUID(t *testing.T) {
	r := regexp.MustCompile("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}$")
	if !r.MatchString(generateUUID().String()) {
		t.Fail()
	}
}

func TestFilePointer(t *testing.T) {
	r := regexp.MustCompile("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}$")
	if !r.MatchString(Key.FilePointer()) {
		t.Fail()
	}
}

func TestSignAndVerify(t *testing.T) {
	hashed := []byte("testing")

	r, s, _, err := Key.Sign(hashed)
	if err != nil {
		t.Fail()
	}

	publicKey, err := Key.getPublicKey()
	if err != nil {
		t.Fail()
	}

	if !Key.Verify(publicKey, hashed, r, s) {
		t.Fail()
	}
}

func TestPrintKey(t *testing.T) {
	t.Skip()
}

func TestMarshall(t *testing.T) {
	t.Skip()
}

func TestUnmarshall(t *testing.T) {
	t.Skip()
}

func TestGetPrivateKey(t *testing.T) {
	pKey, err := Key.getPrivateKey()
	if err != nil {
		t.Fail()
	}

	if pKey == nil {
		t.Fail()
	}
}

func TestGetPublicKey(t *testing.T) {
	pKey, err := Key.getPublicKey()
	if err != nil {
		t.Fail()
	}

	if pKey == nil {
		t.Fail()
	}
}

func TestStruct(t *testing.T) {
	assert.NotNil(t, Key.Struct().GID)
	assert.NotNil(t, Key.Struct().FingerprintMD5)
	assert.NotNil(t, Key.Struct().FingerprintSHA)
}

func TestKeyToGOB64(t *testing.T) {
	gob64, err := keyToGOB64(Key)
	if err != nil {
		t.Logf(err.Error())
		t.Fail()
	}

	key64, err := keyFromGOB64(gob64)
	if err != nil {
		t.Logf(err.Error())
		t.Fail()
	}

	if err := checkFields(Key, key64); err != nil {
		t.Logf(err.Error())
		t.Fail()
	}
}

func TestKeyFromGOB64(t *testing.T) {
	file := fmt.Sprintf("%s/%s/obj.bin", Config.GetString("paths.keys"), Key.FilePointer())
	data, err := helpers.ReadFile(file)
	if err != nil {
		t.Logf(err.Error())
		t.Fail()
	}

	key64, err := keyFromGOB64(data)
	if err != nil {
		t.Logf(err.Error())
		t.Fail()
	}

	if err := checkFields(Key, key64); err != nil {
		t.Logf(err.Error())
		t.Fail()
	}
}

func checkFields(original *key, copied *key) error {
	if original.GID != copied.Struct().GID {
		return fmt.Errorf("failed[GID]")
	}

	if original.FingerprintSHA != copied.FingerprintSHA {
		return fmt.Errorf("failed[FingerprintSHA]")
	}

	if original.FingerprintMD5 != copied.FingerprintMD5 {
		return fmt.Errorf("failed[FingerprintMD5]")
	}

	if original.PrivateKeyB64 != copied.PrivateKeyB64 {
		return fmt.Errorf("failed[PrivateKeyB64]")
	}

	if original.PublicKeyB64 != copied.PublicKeyB64 {
		return fmt.Errorf("failed[PublicKeyB64]")
	}

	if original.PrivateKeyPath != copied.PrivateKeyPath {
		return fmt.Errorf("failed[PrivateKeyPath]")
	}

	if original.PrivatePemPath != copied.PrivatePemPath {
		return fmt.Errorf("failed[PrivatePemPath]")
	}

	return nil
}
