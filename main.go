package main

import (
	"q/cli"
	"q/logs"
)

func main() {
	// Add logs subcommand
	cli.RootCmd.AddCommand(logs.LogsCmd)

	if err := cli.RootCmd.Execute(); err != nil {
		panic(err)
	}
}
