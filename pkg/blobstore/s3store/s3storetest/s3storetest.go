// Package s3storetest reads the settings of the S3-compatible store the tests
// run against (a disposable Garage: scripts/garage_test_server.sh). Without an
// endpoint the tests skip, unless RequireEnv is "true", as in CI.
package s3storetest

import (
	"os"
	"strings"
	"testing"
)

// Environment variables of the test store.
const (
	// EndpointEnv is the URL of the S3 API, e.g. http://127.0.0.1:3910.
	EndpointEnv = "GOELAND_TEST_S3_ENDPOINT"
	// RegionEnv is the region the endpoint signs for.
	RegionEnv = "GOELAND_TEST_S3_REGION"
	// BucketEnv is the existing test bucket.
	BucketEnv = "GOELAND_TEST_S3_BUCKET"
	// AccessKeyIDEnv is the test access key.
	AccessKeyIDEnv = "GOELAND_TEST_S3_ACCESS_KEY_ID"
	// SecretAccessKeyEnv is the test secret key.
	SecretAccessKeyEnv = "GOELAND_TEST_S3_SECRET_ACCESS_KEY"
	// RequireEnv set to "true" turns a missing endpoint into a failure.
	RequireEnv = "GOELAND_REQUIRE_S3_TESTS"
)

// Settings locate the test bucket.
type Settings struct {
	// Endpoint is the URL of the S3 API.
	Endpoint string
	// Region is the signing region.
	Region string
	// Bucket is the test bucket.
	Bucket string
	// AccessKeyID is the test access key.
	AccessKeyID string
	// SecretAccessKey is the test secret key.
	SecretAccessKey string
}

// FromEnv returns the test store settings and whether an endpoint is set.
func FromEnv() (Settings, bool) {
	s := Settings{
		Endpoint: strings.TrimSpace(os.Getenv(EndpointEnv)), Region: os.Getenv(RegionEnv), Bucket: os.Getenv(BucketEnv),
		AccessKeyID: os.Getenv(AccessKeyIDEnv), SecretAccessKey: os.Getenv(SecretAccessKeyEnv),
	}
	return s, s.Endpoint != ""
}

// Require returns the test store settings, or skips the test (fails it when
// RequireEnv is "true"). what names the tests in the message.
func Require(t *testing.T, what string) Settings {
	t.Helper()
	s, ok := FromEnv()
	if ok {
		return s
	}
	if os.Getenv(RequireEnv) == "true" {
		t.Fatalf("%s=true but %s is unset: %s cannot run", RequireEnv, EndpointEnv, what)
	}
	t.Skipf("set %s, %s, %s, %s and %s to run the %s (scripts/garage_test_server.sh)",
		EndpointEnv, RegionEnv, BucketEnv, AccessKeyIDEnv, SecretAccessKeyEnv, what)
	return Settings{}
}
