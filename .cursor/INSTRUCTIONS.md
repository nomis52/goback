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
