package web

import (
	"errors"
	"io"

	"github.com/spf13/cobra"
)

func Command(stdout io.Writer, uiFactory UIFactory) *cobra.Command {
	config := Config{}
	command := &cobra.Command{
		Use:   "ui",
		Short: "Open the local Locus Web interface",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if command.Flags().Changed("json") {
				return errors.New("--json is not supported by ui")
			}
			server, err := New(config, uiFactory)
			if err != nil {
				return err
			}
			return server.ListenAndServe(command.Context(), config.Address, stdout)
		},
	}
	command.Flags().StringVar(&config.Registry, "registry", "", "registry path override")
	command.Flags().StringVar(&config.From, "from", "", "default operational entity")
	command.Flags().StringVar(&config.Vantage, "vantage", "", "default observation vantage")
	command.Flags().StringVar(&config.MechanismBindings, "mechanism-bindings", "", "workstation-local mechanism bindings file")
	command.Flags().StringVar(&config.Address, "address", "127.0.0.1:7070", "loopback listen address")
	return command
}
