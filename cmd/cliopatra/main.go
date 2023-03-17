package main

import (
	"embed"
	clay "github.com/go-go-golems/clay/pkg"
	cmds2 "github.com/go-go-golems/cliopatra/cmd/cliopatra/cmds"
	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/spf13/cobra"
)

//go:embed doc/*
var docFS embed.FS

func main() {
	helpSystem, rootCmd, err := initRootCmd()
	cobra.CheckErr(err)

	lsCmd := cmds2.NewLsCommand()
	rootCmd.AddCommand(lsCmd)

	runCmd := cmds2.NewRunCommand()
	rootCmd.AddCommand(runCmd)

	_ = helpSystem

	err = rootCmd.Execute()
	cobra.CheckErr(err)
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
