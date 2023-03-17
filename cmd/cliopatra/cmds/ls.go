package cmds

import (
	"context"
	"fmt"
	"github.com/go-go-golems/cliopatra/pkg"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/spf13/cobra"
	"strings"
)

type LsProgramCommand struct {
	description *cmds.CommandDescription
}

func (l *LsProgramCommand) Description() *cmds.CommandDescription {
	return l.description
}

func (l *LsProgramCommand) Run(
	ctx context.Context,
	parsedLayers map[string]*layers.ParsedParameterLayer,
	ps map[string]interface{},
	gp cmds.Processor,
) error {
	repositories, ok := ps["repository"].([]string)
	if !ok {
		return fmt.Errorf("repository parameter not found")
	}

	programs := pkg.LoadRepositories(repositories)

	gp.OutputFormatter().AddTableMiddlewareInFront(middlewares.NewReorderColumnOrderMiddleware([]string{"name", "desc", "args"}))

	for _, program := range programs {
		ps_, err2 := program.ComputeArgs(map[string]interface{}{})
		if err2 != nil {
			return err2
		}
		obj := map[string]interface{}{
			"name": program.Name,
			"desc": program.Description,
			"args": strings.Join(ps_, " "),
		}
		err := gp.ProcessInputObject(obj)
		if err != nil {
			return err
		}
	}

	return nil
}

// NewLsCommand returns a new command that lists all the programs available in the repositories.
func NewLsCommand() *cobra.Command {
	glazedParameterLayer, err := cli.NewGlazedParameterLayers()
	cobra.CheckErr(err)

	cmd := &LsProgramCommand{
		description: cmds.NewCommandDescription("ls",
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"repository",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("Repositories to load programs from"),
					parameters.WithRequired(true),
				),
			),
			cmds.WithLayers(glazedParameterLayer),
		),
	}
	cobraCommand, err := cli.BuildCobraCommandFromGlazeCommand(cmd)
	cobra.CheckErr(err)

	return cobraCommand
}
