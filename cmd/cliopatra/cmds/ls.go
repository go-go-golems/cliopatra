package cmds

import (
	"context"
	"github.com/go-go-golems/cliopatra/pkg"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/spf13/cobra"
	"strings"
)

type LsProgramCommand struct {
	*cmds.CommandDescription
}

// NewLsCommand returns a new command that lists all the programs available in the repositories.
func NewLsCommand() *cobra.Command {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	cobra.CheckErr(err)

	cmd := &LsProgramCommand{
		CommandDescription: cmds.NewCommandDescription("ls",
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

type LsCommandSettings struct {
	Repositories []string `glazed.parameter:"repository"`
}

func (l *LsProgramCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &LsCommandSettings{}
	err := parsedLayers.InitializeStruct(layers.DefaultSlug, s)
	if err != nil {
		return err
	}
	r := pkg.NewRepository(s.Repositories)
	err = r.Load()
	if err != nil {
		return err
	}

	for _, program := range r.GetPrograms() {
		ps_, err2 := program.ComputeArgs(parsedLayers.GetAllParsedParameters())
		if err2 != nil {
			return err2
		}
		obj := types.NewRow(
			types.MRP("name", program.Name),
			types.MRP("desc", program.Description),
			types.MRP("args", strings.Join(ps_, " ")),
		)
		err := gp.AddRow(ctx, obj)
		if err != nil {
			return err
		}
	}

	return nil
}
