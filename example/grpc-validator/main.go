package main

import (
	"fmt"
	"os"

	"github.com/zoumo/proxypool/example/grpc-validator/app"
)

func main() {
	command := app.NewCommand()
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
