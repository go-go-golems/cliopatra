package cmds

import (
	"context"
	"fmt"
	"github.com/go-go-golems/cliopatra/pkg"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

// NewRunCommand returns a command that can be used to run either commands from
// a file or from a repository.
//
// It currently doesn't allow overloading flags in the underlying program run
// by cliopatra.
//
// See https://github.com/go-go-golems/glazed/issues/220
func NewRunCommand() *cobra.Command {
	runCommand := &cobra.Command{
		Use:   "run",
		Short: "Run a command from a file or from a repository program",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			repositories, err := cmd.Flags().GetStringSlice("repository")
			cobra.CheckErr(err)

			programs := pkg.LoadRepositories(repositories)

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
