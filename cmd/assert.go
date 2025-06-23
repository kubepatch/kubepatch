package cmd

import (
	"time"

	"github.com/hashmap-kz/kassert/internal/assert"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
)

func NewAssertCmd(streams genericiooptions.IOStreams) *cobra.Command {
	cfgFlags := genericclioptions.NewConfigFlags(true) // all kubectl connection flags
	o := assert.CmdOptions{}

	cmd := &cobra.Command{
		Use:           "assert",
		SilenceErrors: true,
		SilenceUsage:  true,
		Short:         "Run assertions against Kubernetes resources",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return assert.Run(&assert.RunOptions{
				ConfigFlags: cfgFlags,
				Streams:     streams,
				Cmd:         o,
			})
		},
	}

	cmd.Flags().StringVar(&o.File, "file", "", "YAML file with assertions (required)")
	cmd.Flags().IntVar(&o.Retries, "retries", 3, "Number of retries before failing")
	cmd.Flags().DurationVar(&o.Interval, "interval", 2*time.Second, "Polling interval")
	_ = cmd.MarkFlagRequired("file")

	cfgFlags.AddFlags(cmd.Flags())
	return cmd
}
