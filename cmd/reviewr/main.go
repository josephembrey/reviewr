package main

import (
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/app"
	"github.com/josephembrey/reviewr/internal/herdr"
	"github.com/josephembrey/reviewr/internal/preferences"
	"github.com/josephembrey/reviewr/internal/repository"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "reviewr: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 1 {
		return errors.New("usage: reviewr [repository-path]")
	}
	path := ""
	if len(args) == 1 {
		path = args[0]
	}
	repo, err := repository.Open(path)
	if err != nil {
		return err
	}
	host := herdr.NewRuntime(herdr.Detect(os.LookupEnv))
	host.Start()
	defer host.Close()
	stores, err := repo.ScratchStores(os.LookupEnv)
	if err != nil {
		return err
	}
	paneStore, paneState, _ := preferences.Open("")
	model := app.NewWithPaneStateAndScratchScopes(repo, host.Context(), stores, paneStore, paneState.PanesSwapped)
	final, runErr := tea.NewProgram(model).Run()
	model, ok := final.(app.Model)
	if !ok {
		return errors.Join(runErr, stores.Close())
	}
	return errors.Join(runErr, model.Shutdown())
}
