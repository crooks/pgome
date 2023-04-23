package config

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestConfig(t *testing.T) {
	testFile, err := os.CreateTemp("/tmp", "yamn")
	if err != nil {
		t.Fatalf("Unable to create TempFile: %v", err)
	}
	defer os.Remove(testFile.Name())
	testCfg := new(Config)
	testCfg.Logging.Journal = false
	testCfg.Logging.LevelStr = "trace"
	testCfg.API.Username = "user"
	testCfg.API.Password = "password"
	testCfg.API.CertFile = "/foo/file.pem"
	testCfg.API.URL = "https://foobar.baz"
	testCfg.WriteConfig(testFile.Name())
	cfg, err := ParseConfig(testFile.Name())
	if err != nil {
		t.Fatalf("Failed with: %v", err)
	}
	if !cmp.Equal(testCfg, cfg) {
		t.Error("Written config and read config are not equal")
	}
	testCfg.Logging.LevelStr = "breakit"
	if cmp.Equal(testCfg, cfg) {
		t.Error("Written config and read config should *not* be equal")
	}
}

func TestFlags(t *testing.T) {
	f := ParseFlags()
	expectingConfig := "pgome.yml"
	if f.Config != expectingConfig {
		t.Fatalf("Unexpected config flag: Expected=%s, Got=%s", expectingConfig, f.Config)
	}
	if f.Debug {
		t.Fatal("Unexpected debug flag: Expected=false, Got=true")
	}
}
