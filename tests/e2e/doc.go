// Package e2e contains end-to-end tests for the service.
// Unit tests are located near the code they test.
//
// Current tests expect the service to be running on APPTEST_URL (default: http://127.0.0.1:8080).
// Start the service with `docker compose up -d`.
// The tests are also not idempotent and subsequent runs are expected to fail.
// Stop the service and remove data with `docker compose down -v`.
//
// Go will likely mistakenly cache the test results due to lack of the changed source code.
// Run the tests with `go test -count=1 ./tests/e2e/...` to ignore the test cache.
package e2e
