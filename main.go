package main

import (
	"Download_Problem/sim"
	"fmt"
	"os"
)

func main() {
	fmt.Println("=== Download Problem Simulation ===")

	if err := sim.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
