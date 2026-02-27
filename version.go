package main

import (
	"fmt"
	"runtime/debug"
)

type VersionCmd struct{}

func (v *VersionCmd) Run() error {
	fmt.Printf("efmrl3 version %s", version)

	info, ok := debug.ReadBuildInfo()
	if ok {
		var revision, modified string
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				revision = s.Value
			case "vcs.modified":
				modified = s.Value
			}
		}
		if revision != "" {
			if len(revision) > 12 {
				revision = revision[:12]
			}
			fmt.Printf(" (%s", revision)
			if modified == "true" {
				fmt.Print(", modified")
			}
			fmt.Print(")")
		}
	}

	fmt.Println()
	return nil
}
