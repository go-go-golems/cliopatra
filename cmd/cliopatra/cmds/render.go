package cmds

import (
	"context"
	"github.com/go-go-golems/clay/pkg/watcher"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func runWatcher(args []string) {
}

type renderCommandSettings struct {
	Repository      []string `glazed.parameter:"repository"`
	OutputDirectory string   `glazed.parameter:"output-directory"`
	OutputFile      string   `glazed.parameter:"output-file"`
	Watch           bool     `glazed.parameter:"watch"`
	Glob            []string `glazed.parameter:"glob"`
	WithGoTemplate  bool     `glazed.parameter:"with-go-template"`
	WithYamlMarkers bool     `glazed.parameter:"with-yaml-markers"`
}

func NewRenderCommand() *cobra.Command {
	renderLayer, err := layers.NewParameterLayer("render", "Cliopatra rendering options",
		layers.WithFlags(
			parameters.NewParameterDefinition(
				"repository",
				parameters.ParameterTypeStringList,
				parameters.WithHelp("List of repositories to use"),
			),
			parameters.NewParameterDefinition(
				"output-directory",
				parameters.ParameterTypeString,
				parameters.WithHelp("Output directory"),
			),
			parameters.NewParameterDefinition(
				"output-file",
				parameters.ParameterTypeString,
				parameters.WithHelp("Output file"),
			),
			parameters.NewParameterDefinition(
				"watch",
				parameters.ParameterTypeBool,
				parameters.WithHelp("Watch for changes"),
				parameters.WithDefault(false),
			),
			parameters.NewParameterDefinition(
				"glob",
				parameters.ParameterTypeStringList,
				parameters.WithHelp("List of doublestar file glob"),
				parameters.WithDefault([]string{"**.tmpl.md"}),
			),
			parameters.NewParameterDefinition(
				"with-go-template",
				parameters.ParameterTypeBool,
				parameters.WithHelp("Use go template"),
				parameters.WithDefault(true),
			),
			parameters.NewParameterDefinition(
				"with-yaml-markers",
				parameters.ParameterTypeBool,
				parameters.WithHelp("Recognize yaml markers"),
				parameters.WithDefault(true),
			),
		),
	)
	cobra.CheckErr(err)

	description := cmds.NewCommandDescription("render",
		cmds.WithLong("Render a go template file by expanding cliopatra calls"),
		cmds.WithLayers(renderLayer),
		cmds.WithArguments(
			parameters.NewParameterDefinition(
				"files",
				parameters.ParameterTypeStringList,
				parameters.WithHelp("List of files or directories to render"),
				parameters.WithRequired(true),
			),
		),
	)

	cobraParser, err := cli.NewCobraParserFromCommandDescription(description)
	cobra.CheckErr(err)
	renderCommand := cobraParser.Cmd

	renderCommand.Run = func(cmd *cobra.Command, args []string) {
		parsedLayers, ps, err := cobraParser.Parse(args)
		cobra.CheckErr(err)

		renderLayer, ok := parsedLayers["render"]
		if !ok {
			cobra.CheckErr(errors.New("render layer not found"))
		}
		settings := &renderCommandSettings{}
		err = parameters.InitializeStructFromParameters(settings, renderLayer.Parameters)
		cobra.CheckErr(err)

		files, ok := ps["files"]
		if !ok {
			cobra.CheckErr(errors.New("files parameter not found"))
		}
		files_, ok := files.([]string)
		if !ok {
			cobra.CheckErr(errors.New("files parameter is not a string list"))
		}

		watcherOptions := []watcher.Option{
			watcher.WithPaths(files_...),
		}

		if settings.Glob != nil && len(settings.Glob) > 0 {
			watcherOptions = append(watcherOptions, watcher.WithMask(settings.Glob...))
		}

		if settings.Watch {
			w := watcher.NewWatcher(func(path string) error {
				log.Info().Str("path", path).Msg("File changed")
				return nil
			},
				watcher.WithPaths(files_...),
				watcher.WithMask("**/*.txt"))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			eg := errgroup.Group{}
			eg.Go(func() error {
				return w.Run(ctx)
			})

			err := eg.Wait()
			// check that the error wasn't a cancel
			if err != nil && err != context.Canceled {
				log.Error().Err(err).Msg("Error running watcher")
			}
			cobra.CheckErr(err)

			runWatcher(files.([]string))
			return
		}

		panic("not implemented")
	}

	// arguments: List of directories to render
	// flags:
	// - output directory
	// - watch mode
	// - file glob
	// - use go template
	// - recognize yaml markers
	// - custom markers ??

	// if we were to use a glaze.Command to do this, we'd probably want the type
	// that emits structured data over a channel, since it would be used to display progress in a console
	// or web UI, for example

	return renderCommand
}
