package main

import (
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/app"
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
	_, err = tea.NewProgram(app.New(repo)).Run()
	return err
}
