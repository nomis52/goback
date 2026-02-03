# TESTING.md

This document describes the testing patterns and requirements for this project.

## General Principles

- **Test Files Location**: Place test files (`*_test.go`) in the same directory as the code they test.
- **Package Names**: Test files must use the same package name as the code they test.
- **Blackbox Testing**: Only test exported methods and types. Never test unexported fields or internal state (whitebox testing).
- **No Sleeps**: Never use `time.Sleep` in unit tests as they cause flakiness. Use appropriate synchronization or polling if necessary.
- **Mock Domains**: Use RFC 6761 reserved domains (e.g., `.test`, `.example`, `.invalid`, `.localhost`) for mock hostnames in tests.

## Assertions

- **Use Testify**: Always use `github.com/stretchr/testify` for assertions.
- **Require vs Assert**:
    - Use `require` (e.g., `require.NoError`, `require.Error`) for preconditions or critical checks where the test should fail fast and not continue.
    - Use `assert` for the actual test verifications.
- **Exact Matches**: Prefer `assert.Equal` with complete expected values. Use `assert.Contains` only if the content varies by environment (e.g., specific file paths or system-dependent error messages).

## Table-Driven Tests

Table-driven tests are the required pattern for organizing multiple test cases.

- **Error Checking**: Use a `wantErr string` field in the test case struct to check both the presence and content of an error.
    - If `wantErr` is empty, no error is expected (`assert.NoError`).
    - If `wantErr` is non-empty, an error is expected and its message must be verified (preferring `assert.Equal` as noted above).
- **Verification Functions**: Use a `verifyFn` field (e.g., `verifyFn func(t *testing.T, result Type)`) for complex success-case assertions. Do not use `verifyFn` for simple checks like boolean flags.

## Mocking and Dependencies

- **Injectable Dependencies**: Make external dependencies injectable. Wrap external calls (e.g., `exec.Command`, network requests) in interfaces that can be swapped for mocks in tests.
- **Manual Mocks**: Prefer simple manual mocks, fakes, or stubs over complex mocking frameworks. They are easier to maintain and understand.
- **Placement**: Keep test helper types (mocks, stubs, test fixtures) and their methods at the end of the test file, after all test functions.

## Test Commands

```bash
# Run all tests
make test
# or
go test ./...

# Run tests for a specific package
go test ./orchestrator

# Run a single test
go test -v -run TestName ./path/to/package
```
