package main

import (
	"context"
	"fmt"
	"os"

	"github.com/tungnguyensipher/callmeback/internal/cli"
)

func main() {
	if err := cli.Execute(context.Background(), cli.Options{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
