package main

import (
	"context"
	"fmt"
	"goshare/cmd"
	"os"
)

func main() {
	cmd.Prerun(context.Background())
	fmt.Fprintln(os.Stdout, "Shutdown and cleanup of resources complete")
}
