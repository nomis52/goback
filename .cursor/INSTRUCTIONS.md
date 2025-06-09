# Cursor Coding Instructions

1. All program logic should be inside `doMain()`.
2. The `main()` function must only call `doMain()` and handle its error (e.g., print to stderr and exit with a nonzero code).
3. Argument parsing must be handled in a separate function called `parseArgs()`, which returns an `Args` struct containing all parsed command-line arguments.
4. `doMain()` must call `parseArgs()` at the start and use the returned `Args` struct for further logic.
5. The `Args` struct should be defined at the top level of the file. All command-line arguments should be fields in this struct.
6. No argument parsing should occur in `main()` or directly in `doMain()`; always use `parseArgs()`.
7. If you add new command-line arguments, update the `Args` struct and `parseArgs()` accordingly.

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
- `main()` → only calls `doMain()`
- `doMain()` → calls `parseArgs()` and contains all logic
- `parseArgs()` → returns an `Args` struct with all parsed arguments 