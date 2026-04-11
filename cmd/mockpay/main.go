// Command mockpay is a local, provider-agnostic payment simulator CLI.
// See the project README for usage.
package main

import (
	"os"

	"github.com/yjmrobert/txsim/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
