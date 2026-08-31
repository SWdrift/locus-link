package web

import (
	"errors"
	"io"

	"github.com/spf13/cobra"
)

func Command(stdout io.Writer) *cobra.Command {
	config := Config{}
	command := &cobra.Command{
		Use:   "web",
		Short: "Open the local Locus Web interface",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if command.Flags().Changed("json") {
				return errors.New("--json is not supported by web")
			}
			server, err := New(config)
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
