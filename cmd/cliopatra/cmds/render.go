package cmds

import (
	"context"
	"github.com/go-go-golems/clay/pkg/watcher"
	"github.com/go-go-golems/cliopatra/pkg"
	"github.com/go-go-golems/cliopatra/pkg/render"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
)

type renderCommandSettings struct {
	Repository           []string          `glazed.parameter:"repository"`
	OutputDirectory      string            `glazed.parameter:"output-directory"`
	OutputFile           string            `glazed.parameter:"output-file"`
	Watch                bool              `glazed.parameter:"watch"`
	Glob                 []string          `glazed.parameter:"glob"`
	WithGoTemplate       bool              `glazed.parameter:"with-go-template"`
	WithYamlMarkers      bool              `glazed.parameter:"with-yaml-markers"`
	Delimiters           []string          `glazed.parameter:"delimiters"`
	AllowProgramCreation bool              `glazed.parameter:"allow-program-creation"`
	Quiet                bool              `glazed.parameter:"quiet"`
	RenameOutputFiles    map[string]string `glazed.parameter:"rename-output-files"`
	BaseDirectory        string            `glazed.parameter:"base-directory"`
}

func NewRenderCommand() *cobra.Command {
	renderLayer, err := layers.NewParameterLayer("render", "Cliopatra rendering options",
		layers.WithFlags(
			parameters.NewParameterDefinition(
				"repository",
				parameters.ParameterTypeStringList,
				parameters.WithHelp("List of repositories to use"),
				parameters.WithDefault([]string{}),
			),
			parameters.NewParameterDefinition(
				"output-directory",
				parameters.ParameterTypeString,
				parameters.WithHelp("Output directory"),
				parameters.WithDefault("."),
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
				parameters.WithDefault([]string{"**/*.tmpl.md"}),
			),
			parameters.NewParameterDefinition(
				"rename-output-files",
				parameters.ParameterTypeKeyValue,
				parameters.WithHelp("Rename output files"),
				parameters.WithDefault(map[string]string{
					"tmpl.md": "md",
				}),
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
			parameters.NewParameterDefinition(
				"delimiters",
				parameters.ParameterTypeStringList,
				parameters.WithHelp("Left and right delimiter, separated by ,"),
				parameters.WithDefault([]string{"{{", "}}"}),
			),
			parameters.NewParameterDefinition(
				"allow-program-creation",
				parameters.ParameterTypeBool,
				parameters.WithHelp("Allow program creation"),
				parameters.WithDefault(false),
			),
			parameters.NewParameterDefinition(
				"quiet",
				parameters.ParameterTypeBool,
				parameters.WithHelp("Quiet mode"),
				parameters.WithDefault(false),
			),
			parameters.NewParameterDefinition(
				"base-directory",
				parameters.ParameterTypeString,
				parameters.WithHelp("Base directory"),
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

		repositories := ps["repository"].([]string)
		repository := pkg.NewRepository(repositories)
		err = repository.Load()
		cobra.CheckErr(err)

		files, ok := ps["files"]
		if !ok {
			cobra.CheckErr(errors.New("files parameter not found"))
		}
		files_, ok := files.([]string)
		if !ok {
			cobra.CheckErr(errors.New("files parameter is not a string list"))
		}

		if settings.Delimiters != nil && len(settings.Delimiters) != 2 {
			cobra.CheckErr(errors.New("delimiters parameter must have 2 values"))
		}

		// Create the renderer, now that we gathered all the options
		options := []render.Option{
			render.WithRepositories(repository),
			render.WithGoTemplate(settings.WithGoTemplate),
			render.WithYamlMarkers(settings.WithYamlMarkers),
			render.WithAllowProgramCreation(settings.AllowProgramCreation),
			render.WithVerbose(!settings.Quiet),
		}
		if settings.Glob != nil {
			options = append(options, render.WithMasks(settings.Glob...))
		}

		if settings.Delimiters != nil {
			options = append(options, render.WithDelimiters(settings.Delimiters[0], settings.Delimiters[1]))
		}

		if settings.OutputDirectory != "" {
			options = append(options, render.WithRenameOutputFiles(settings.RenameOutputFiles))
		}

		renderer := render.NewRenderer(options...)

		if settings.OutputFile != "" && len(files_) > 1 {
			cobra.CheckErr(errors.New("output-file parameter can only be used with a single file"))
		}

		if settings.OutputDirectory != "" && !strings.HasSuffix(settings.OutputDirectory, "/") {
			settings.OutputDirectory += "/"
		}

		// fimd all directories given on the command line, and make sure they have a / at the end
		dirs := []string{}
		for _, file := range files_ {
			fi, err := os.Stat(file)
			cobra.CheckErr(err)
			if fi.IsDir() {
				d := file
				if !strings.HasSuffix(d, "/") {
					d += "/"
				}
				dirs = append(dirs, d)
			}
		}

		for _, file := range files_ {
			// check if file is a directory
			fi, err := os.Stat(file)
			cobra.CheckErr(err)

			if fi.IsDir() {
				if settings.OutputDirectory == "" {
					cobra.CheckErr(errors.New("output-directory parameter is required when rendering a directory"))
				}

				err = renderer.RenderDirectory(file, settings.OutputDirectory)
				cobra.CheckErr(err)

			} else {
				f, err := os.Open(file)
				cobra.CheckErr(err)
				defer func(f *os.File) {
					_ = f.Close()
				}(f)

				var outputFile string
				if settings.OutputFile != "" {
					outputFile = settings.OutputFile
				} else {
					basePath := render.ComputeBaseDirectory(file, dirs, settings.BaseDirectory)
					outputFile = filepath.Join(
						settings.OutputDirectory,
						strings.TrimPrefix(file, basePath),
					)
				}

				err = renderer.RenderFile(file, outputFile)
				cobra.CheckErr(err)
			}
		}

		if settings.Watch {
			outputDirectory_, ok := ps["output-directory"]
			if !ok {
				cobra.CheckErr(errors.New("output-directory parameter not found"))
			}

			outputDirectory, ok := outputDirectory_.(string)
			if !ok {
				cobra.CheckErr(errors.New("output-directory parameter is not a string"))
			}

			if outputDirectory == "" {
				cobra.CheckErr(errors.New("output-directory parameter is empty"))
			}

			watcherOptions := []watcher.Option{
				watcher.WithWriteCallback(
					func(path string) error {
						basePath := render.ComputeBaseDirectory(path, files_, settings.BaseDirectory)
						outputPath := filepath.Join(outputDirectory, strings.TrimPrefix(path, basePath))

						log.Debug().
							Str("path", path).
							Str("basePath", basePath).
							Str("outputPath", outputPath).
							Msg("File changed")

						err = renderer.RenderFile(path, outputPath)
						if err != nil {
							log.Error().Err(err).Msg("Error rendering file")
						}

						return nil
					}),
				watcher.WithRemoveCallback(func(path string) error {
					basePath := render.ComputeBaseDirectory(path, files_, settings.BaseDirectory)
					outputPath := filepath.Join(outputDirectory, strings.TrimPrefix(path, basePath))

					for k, v := range settings.RenameOutputFiles {
						if strings.HasSuffix(outputPath, k) {
							outputPath = strings.TrimSuffix(outputPath, k) + v
							break
						}
					}

					log.Debug().
						Str("path", path).
						Str("basePath", basePath).
						Str("outputPath", outputPath).
						Msg("File removed")

					err = os.Remove(outputPath)
					if err != nil {
						log.Error().Err(err).Msg("Error removing file")
					}
					return nil
				}),
				watcher.WithPaths(files_...),
			}

			if settings.Glob != nil && len(settings.Glob) > 0 {
				watcherOptions = append(watcherOptions, watcher.WithMask(settings.Glob...))
			}

			w := watcher.NewWatcher(watcherOptions...)

			eg, ctx := errgroup.WithContext(context.Background())
			ctx2, cancel := context.WithCancel(ctx)
			defer cancel()
			ctx2, stop := signal.NotifyContext(ctx2, os.Interrupt)
			defer stop()

			eg.Go(func() error {
				log.Info().Msg("Starting watcher")
				return w.Run(ctx2)
			})
			eg.Go(func() error {
				return repository.Watch(ctx2)
			})
			cobra.CheckErr(err)

			err := eg.Wait()
			// check that the error wasn't a cancel
			if err != nil && err != context.Canceled {
				log.Error().Err(err).Msg("Error running watcher")
				cobra.CheckErr(err)
			}
		}

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
