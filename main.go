package main

import "github.com/hokupod/sitepanda/cmd"

func main() {
	// Set the handlers for the cmd package
	cmd.InitHandler = HandleInitCommand
	cmd.ScrapingHandler = HandleScraping
	cmd.VersionFunc = func() string { return Version }

	cmd.Execute()
}
