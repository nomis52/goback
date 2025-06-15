# Cursor Coding Instructions

## Code Style

1. **Follow the Uber Go Style Guide: https://github.com/uber-go/guide/blob/master/style.md**
   - Use the Uber Go Style Guide as the primary reference for all Go code
   - When in doubt about style, formatting, or best practices, consult the Uber guide
   - This includes naming conventions, error handling, channel usage, struct organization, etc.
   - The rules below supplement but do not override the Uber guide

2. **Time values must be defined as constants**
   - All time durations, intervals, and timeouts must be defined as constants
   - Use descriptive names that indicate the purpose of the time value
   - Example:
     ```go
     const (
         defaultBootTimeout = 5 * time.Minute
         pingCheckInterval  = 5 * time.Second
     )
     ```
   - This applies to both default values and implementation details
   - The only exception is when the time value is purely configurable and has no default

## Go Testing Style (Always Prefer Testify)

- **Always use [testify](https://github.com/stretchr/testify) for assertions and requirements in all Go tests.**
  - Use `require` for checks that should halt the test on failure (e.g., setup, critical preconditions).
  - Use `assert` for checks where the test can continue after a failure.
- Do **not** use the standard library's `t.Errorf`, `t.Fatal`, or similar assertion patterns unless absolutely necessary.
- Import as:
  ```go
  import (
      "github.com/stretchr/testify/assert"
      "github.com/stretchr/testify/require"
  )
  ```
- Prefer expressive, one-line assertions:
  ```go
  require.NoError(t, err)
  assert.Equal(t, expected, actual)
  assert.Len(t, slice, 3)
  ```

## Program Structure

2. All program logic should be inside `run()`.
3. The `main()` function must only call `run()` and handle its error (e.g., print to stderr and exit with a nonzero code).
4. Argument parsing must be handled in a separate function called `parseArgs()`, which returns an `Args` struct containing all parsed command-line arguments.
5. `run()` must call `parseArgs()` at the start and use the returned `Args` struct for further logic.
6. The `Args` struct should be defined at the top level of the file. All command-line arguments should be fields in this struct.
7. No argument parsing should occur in `main()` or directly in `run()`; always use `parseArgs()`.
8. If you add new command-line arguments, update the `Args` struct and `parseArgs()` accordingly.

## Documentation Standards

11. **All documentation must be in godoc comments - never create separate .md files for API contracts or documentation.**
    - Use package-level comments in `doc.go` for comprehensive package documentation
    - Document API contracts, usage patterns, and examples directly in godoc comments
    - Include state diagrams, behavior specifications, and error handling patterns in package documentation
    - For complex packages, create a `doc.go` file with extensive package-level documentation
    - Method and type documentation should be complete and self-contained
    - **Exception:** Only README.md files for project overview and setup instructions are allowed

## File Organization

9. **Test files must be placed in the same directory as the code they test.** 
   - Tests for `orchestrator/orchestrator.go` go in `orchestrator/orchestrator_test.go`
   - Tests for `ipmi/ipmi.go` go in `ipmi/ipmi_test.go`
   - **Never** place test files in parent directories or separate test directories
   - Test files should use the same package name as the code they test
   - Example: tests in `orchestrator/` should have `package orchestrator`

10. **The directory name must always match the package name.**
    - For example, code in the `pbsclient` package must be in the `pbsclient/` directory.
    - This ensures consistency and clarity in the codebase structure.

## Example Template
```go
type Args struct {
    ConfigPath string
    // Add more fields as needed
}

func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}

func parseArgs() Args {
    // Parse flags
    // Return Args struct
}

func run() error {
    args := parseArgs()
    // All main logic here, using args
}
```

**Summary:**
- **Follow the Uber Go Style Guide for all code style decisions**
- `main()` → only calls `run()`
- `run()` → calls `parseArgs()` and contains all logic
- `parseArgs()` → returns an `Args` struct with all parsed arguments
- **Tests go in the same directory as the code being tested**

## Client Package Layout & Naming Conventions

- **Package Name:**  
  Use the form `fooclient` (e.g., `pbsclient`, `barclient`).

- **Directory Name:**  
  The directory must match the package name (e.g., `pbsclient/`, `barclient/`).

- **Main Type:**  
  The main exported type must be `Client`.

- **Constructor:**  
  Provide a `New()` function (e.g., `fooclient.New(...)`) that returns a pointer to `Client`.

- **Test Files:**  
  - Place test files in the same directory as the implementation.
  - Test files must use the same package name as the code they test (e.g., `package pbsclient`).

- **Documentation:**  
  - Document the package, `Client` type, constructor, and all exported methods using Go-style comments.
  - Include a brief example usage in the package comment.
  - When writing documentation for methods, focus on what action is performed, rather than how the method is implemented.
  - The documentation on each method & type should specify the contract, don't document the contract in separate markdown files.
  - **ALL documentation must be in godoc comments - never create separate .md files for API contracts or documentation.**

- **General Rule:**  
  - The directory name must always match the package name.
  - The package name must always be in the form `fooclient` for service clients.

## Making Code Changes

When making code changes, follow these guidelines:

1. Use the Options pattern for Go constructors that take multiple parameters:
   - Define an `Option` type as a function that modifies the struct
   - Create option functions with the `With` prefix (e.g., `WithTimeout`, `WithPrefix`)
   - Make the constructor accept variadic options
   - Set sensible defaults in the constructor
   - Document each option function clearly

Example:
```go
type Option func(*Client)

func WithTimeout(timeout time.Duration) Option {
    return func(c *Client) {
        c.timeout = timeout
    }
}

func NewClient(url string, opts ...Option) *Client {
    client := &Client{
        timeout: DefaultTimeout,
    }
    for _, opt := range opts {
        opt(client)
    }
    return client
}
```

2. Use constants for default values and magic numbers
3. Use standard library constants instead of string literals (e.g., `http.MethodPost` instead of "POST")
4. Only accept HTTP 200 responses unless there's a specific reason to accept other status codes
5. Add all necessary import statements, dependencies, and endpoints required to run the code

## Calling External APIs

When selecting which version of an API or package to use, choose one that is compatible with the USER's dependency management file. If an external API requires an API Key, be sure to point this out to the USER. Adhere to best security practices (e.g. DO NOT hardcode an API key in a place where it can be exposed)
