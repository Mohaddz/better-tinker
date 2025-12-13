package main

import (
	"fmt"
	"os"

	"github.com/mohadese/tinker-cli/internal/tui"
)

func main() {
	p := tui.NewProgram()
	if _, err := p.Run(); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}
