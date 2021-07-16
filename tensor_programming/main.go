package main

import (
	"blockchain_learn/tensor_programming/cli"
	"os"
)

func main() {
	defer os.Exit(0)
	cmd := cli.CommandLine{}
	cmd.Run()
}
