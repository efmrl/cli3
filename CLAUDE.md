This is a project to create a CLI for my "efmrl" project. The efmrl project provides hosting for ephemeral web sites. These sites are static sites: no data is stored by users. The owner of the site may, of course, populate the site with HTML, CSS, JavaScript, images, etc. That's what the CLI does: it allows the site owner to sync their changes to the hosted efmrl site.

The CLI will be written in Go. It will use "kong", https://pkg.go.dev/github.com/alecthomas/kong, to parse the command line options, set defaults, read override defaults from the environment, etc.

The CLI will have a config file that describes the efmrl site it's hosting for, and how to sync their changes. This config file will typically be kept with the source for the site. For example, if the site is a hugo site, then along with a hugo.toml file, we will have a config file for syncing the site to efmrl. There will also be credentials that are not kept in the config. These will be kept somewhere in the $HOME/.config/efmrl3 directory. By keeping these things outside the "current" directory, we avoid storing credentials in (for example) github.

The efmrl server uses WorkOS (https://workos.com) for authentication. Let's try to use the "Device Authentication Flow" to authenticate our CLI with the server. The user will be directed in their browser to log in if necessary, and confirm or deny that the CLI should be granted authorization to act on the user's behalf.

The CLI should be hierarchical. If the command is called "efmrl3", then example commands might be "efmrl3 status", "efmrl3 configure", "efmrl3 login", and "efmrl3 sync".

We will use homebrew (https://brew.sh/) to distribute this executable to mac and certain linux platforms. We already have a github repo at https://github.com/efmrl/homebrew-cli for our old efmrl CLI. Our new one will be called efmrl3, at least at first. We might eventually delete the existing "efmrl", and rename "efmrl3" to be just "efmrl". Goreleaser (https://goreleaser.com/) takes care of the details of how to make a homebrew tap for our CLI.
