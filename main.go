package main

import (
    "fmt"
    "os"

    "NyteBubo/cmd"
)

func main() {
    // Execute the root Cobra command
    if err := cmd.Execute(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}