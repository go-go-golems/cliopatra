package main

import (
	"context"
	"embed"
	"fmt"
	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/cliopatra/pkg"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

//go:embed doc/*
var docFS embed.FS

func main() {
	helpSystem, rootCmd, err := initRootCmd()
	cobra.CheckErr(err)

	lsCmd := newLsCommand()
	rootCmd.AddCommand(lsCmd)

	runCmd := newRunCommand()
	rootCmd.AddCommand(runCmd)

	_ = helpSystem

	err = rootCmd.Execute()
	cobra.CheckErr(err)
}

// newRunCommand returns a command that can be used to run either commands from
// a file or from a repository.
//
// It currently doesn't allow overloading flags in the underlying program run
// by cliopatra.
//
// See https://github.com/go-go-golems/glazed/issues/220
func newRunCommand() *cobra.Command {
	runCommand := &cobra.Command{
		Use:   "run",
		Short: "Run a command from a file or from a repository program",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			repositories, err := cmd.Flags().GetStringSlice("repository")
			cobra.CheckErr(err)

			programs := loadRepositories(repositories)

			file, err := cmd.Flags().GetString("file")
			cobra.CheckErr(err)
			program, err := cmd.Flags().GetString("program")
			cobra.CheckErr(err)

			options := len(args)
			if file != "" {
				options++
			}
			if program != "" {
				options++
			}

			if options > 1 {
				cobra.CheckErr(fmt.Errorf("cannot specify both file and program"))
			}

			var p *pkg.Program

			if file != "" {
				f, err := os.Open(file)
				cobra.CheckErr(err)
				defer f.Close()
				p, err = pkg.NewProgramFromYAML(f)
				cobra.CheckErr(err)
			}

			if program != "" {
				p_, ok := programs[program]
				if !ok {
					cobra.CheckErr(fmt.Errorf("program %s not found", program))
				}
				p = p_
			}

			if len(args) > 0 {
				// check if args[0] is a yaml file, otherwise treat as program name
				f, err := os.Open(args[0])
				if err == nil {
					defer f.Close()
					p, err = pkg.NewProgramFromYAML(f)
					cobra.CheckErr(err)
				} else {
					p_, ok := programs[args[0]]
					if !ok {
						cobra.CheckErr(fmt.Errorf("program %s not found", args[0]))
					}
					p = p_
				}
			}

			if p == nil {
				cobra.CheckErr(fmt.Errorf("either file or program must be specified"))
			}

			// TODO(manuel, 2023-03-17) To allow the user to override flags of the loaded cliopatra program
			// we need to use a similar mechanism to what sqleton does with its run-command hack.
			// We basically need to handle flags manually until we know which program we are going to run,
			// at that point we can initialize a custom cobra command with the necessary flags,
			// and run that instead after doing some os.Args splicing.
			//
			// See https://github.com/go-go-golems/glazed/issues/220
			ctx := context.Background()
			buf := &strings.Builder{}
			err = p.RunIntoWriter(
				ctx,
				map[string]*layers.ParsedParameterLayer{},
				map[string]interface{}{},
				buf,
			)
			cobra.CheckErr(err)

			fmt.Println(buf.String())
		},
	}

	runCommand.Flags().StringSlice("repository", []string{}, "Repository to load commands from")
	runCommand.Flags().String("file", "", "File to load commands from")
	runCommand.Flags().String("program", "", "Name of the program loaded from the repositories")

	return runCommand
}

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

	programs := loadRepositories(repositories)

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

// Returns a new command that lists all the programs available in the repositories.
func newLsCommand() *cobra.Command {
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
	cobraCommand, err := cli.BuildCobraCommand(cmd)
	cobra.CheckErr(err)

	return cobraCommand
}

func loadRepositories(repositories []string) map[string]*pkg.Program {
	programs := map[string]*pkg.Program{}

	for _, repository := range repositories {
		_, err := os.Stat(repository)
		programs_, err := pkg.LoadProgramsFromFS(os.DirFS(repository), ".")
		cobra.CheckErr(err)

		for _, program := range programs_ {
			if _, ok := programs[program.Name]; ok {
				cobra.CheckErr(fmt.Errorf("program %s already exists", program.Name))
			}
			programs[program.Name] = program
		}
	}
	return programs
}

func initRootCmd() (*help.HelpSystem, *cobra.Command, error) {
	helpSystem := help.NewHelpSystem()
	err := helpSystem.LoadSectionsFromFS(docFS, ".")
	cobra.CheckErr(err)

	rootCmd := &cobra.Command{
		Use:   "cliopatra",
		Short: "cliopatra is a tool to run and test CLI applications",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Reinitalize the logger after the config has been loaded
			err = clay.InitLogger()
			cobra.CheckErr(err)
		},
	}

	helpSystem.SetupCobraRootCommand(rootCmd)

	err = clay.InitViper("cliopatra", rootCmd)
	cobra.CheckErr(err)
	err = clay.InitLogger()
	cobra.CheckErr(err)

	return helpSystem, rootCmd, nil
}
