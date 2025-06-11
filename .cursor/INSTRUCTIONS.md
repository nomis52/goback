# Cursor Coding Instructions

## Code Style

1. **Follow the Uber Go Style Guide: https://github.com/uber-go/guide/blob/master/style.md**
   - Use the Uber Go Style Guide as the primary reference for all Go code
   - When in doubt about style, formatting, or best practices, consult the Uber guide
   - This includes naming conventions, error handling, channel usage, struct organization, etc.
   - The rules below supplement but do not override the Uber guide

## Program Structure

2. All program logic should be inside `doMain()`.
3. The `main()` function must only call `doMain()` and handle its error (e.g., print to stderr and exit with a nonzero code).
4. Argument parsing must be handled in a separate function called `parseArgs()`, which returns an `Args` struct containing all parsed command-line arguments.
5. `doMain()` must call `parseArgs()` at the start and use the returned `Args` struct for further logic.
6. The `Args` struct should be defined at the top level of the file. All command-line arguments should be fields in this struct.
7. No argument parsing should occur in `main()` or directly in `doMain()`; always use `parseArgs()`.
8. If you add new command-line arguments, update the `Args` struct and `parseArgs()` accordingly.

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
    if err := doMain(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}

func parseArgs() Args {
    // Parse flags
    // Return Args struct
}

func doMain() error {
    args := parseArgs()
    // All main logic here, using args
}
```

**Summary:**
- **Follow the Uber Go Style Guide for all code style decisions**
- `main()` → only calls `doMain()`
- `doMain()` → calls `parseArgs()` and contains all logic
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
