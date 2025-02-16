// Package e2e contains end-to-end tests for the service.
// Unit tests are located near the code they test.
//
// Current tests expect the service to be running on http://127.0.0.1:8080.
// Start the service with `docker compose up -d`.
//
// Go will likely mistakenly cache the test results due to lack of the changed source code.
// Run the tests with `go test -count=1 ./tests/...` to ignore the test cache.
package e2e
