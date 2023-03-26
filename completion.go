package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var supportedShells = []string{
	"bash",
	"zsh",
	"fish",
	"powershell",
}

func printCompletionScript(cmd *cobra.Command, streams genericclioptions.IOStreams, shell string) error {
	switch shell {
	case "bash":
		return cmd.Root().GenBashCompletion(streams.Out)
	case "zsh":
		return cmd.Root().GenZshCompletion(streams.Out)
	case "fish":
		return cmd.Root().GenFishCompletion(streams.Out, true)
	case "powershell":
		return cmd.Root().GenPowerShellCompletionWithDesc(streams.Out)
	}

	return fmt.Errorf("shell not supported: %s", shell)
}
