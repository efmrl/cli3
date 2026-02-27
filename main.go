package main

import (
	"github.com/alecthomas/kong"
)

// version is set at build time via goreleaser ldflags (-X main.version=...)
var version = "dev"

var CLI struct {
	Status   StatusCmd   `cmd:"" help:"Show site status and configuration"`
	Config   ConfigCmd   `cmd:"" help:"View or modify configuration"`
	Login    LoginCmd    `cmd:"" help:"Authenticate with efmrl server"`
	Logout   LogoutCmd   `cmd:"" help:"Clear authentication credentials"`
	Sync     SyncCmd     `cmd:"" help:"Synchronize local files with remote site"`
	Domains  DomainsCmd  `cmd:"" help:"Manage domains for this efmrl"`
	Rewrites RewritesCmd `cmd:"" help:"Manage rewrites for this efmrl"`
	Version  VersionCmd  `cmd:"" help:"Print version information"`
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
