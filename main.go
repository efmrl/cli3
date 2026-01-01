package main

import (
	"fmt"

	"github.com/alecthomas/kong"
)

var CLI struct {
	Hello    HelloCmd    `cmd:"" help:"Say hello world"`
	Status   StatusCmd   `cmd:"" help:"Show site status and configuration"`
	Config   ConfigCmd   `cmd:"" help:"View or modify configuration"`
	Login    LoginCmd    `cmd:"" help:"Authenticate with efmrl server"`
	Logout   LogoutCmd   `cmd:"" help:"Clear authentication credentials"`
	Sync     SyncCmd     `cmd:"" help:"Synchronize local files with remote site"`
	Domains  DomainsCmd  `cmd:"" help:"Manage domains for this efmrl"`
	Rewrites RewritesCmd `cmd:"" help:"Manage rewrites for this efmrl"`
}

type HelloCmd struct{}

func (h *HelloCmd) Run() error {
	fmt.Println("hello mister zebra")
	return nil
}

func main() {
	ctx := kong.Parse(&CLI,
		kong.Name("efmrl3"),
		kong.Description("CLI for efmrl ephemeral web site hosting"),
		kong.UsageOnError(),
	)
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
