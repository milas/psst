package main

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	k8sterm "k8s.io/kubectl/pkg/util/term"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cqroot/prompt"
	"github.com/cqroot/prompt/choose"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	k8scmd "k8s.io/kubectl/pkg/cmd/util"
	utilcomp "k8s.io/kubectl/pkg/util/completion"
)

//go:embed usage.txt
var UsageMessage string

type Options struct {
	SecretName string
	SecretKey  string

	Raw bool
}

func NewCommand(streams genericclioptions.IOStreams) (*cobra.Command, k8scmd.Factory) {
	var kubeFactory k8scmd.Factory
	var raw bool
	var autocompleteShellLanguage string

	cmd := &cobra.Command{
		Use:  "psst [--namespace NAMESPACE] [NAME [KEY]]",
		Long: fmt.Sprintf(UsageMessage, "psst"),
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if autocompleteShellLanguage != "" {
				return printCompletionScript(cmd, streams, autocompleteShellLanguage)
			}

			opts := Options{Raw: raw}
			if len(args) != 0 {
				opts.SecretName = args[0]
			}
			if len(args) == 2 {
				opts.SecretKey = args[1]
			}

			secretClient, err := secretClient(kubeFactory)
			if err != nil {
				return err
			}
			return Run(ctx, streams, secretClient, opts)
		},
		ValidArgsFunction: func(
			cmd *cobra.Command,
			args []string,
			toComplete string,
		) ([]string, cobra.ShellCompDirective) {
			ctx := cmd.Context()
			cobra.CompDebugln(fmt.Sprintf("cmd::args: %v", cmd.Flags().Args()), true)
			cobra.CompDebugln(fmt.Sprintf("args: %v", args), true)
			cobra.CompDebugln(fmt.Sprintf("toComplete: %s", toComplete), true)
			const defaultFlags = cobra.ShellCompDirectiveNoFileComp
			if len(args) == 0 {
				secretClient, err := secretClient(kubeFactory)
				if err != nil {
					cobra.CompErrorln(err.Error())
					return nil, defaultFlags | cobra.ShellCompDirectiveError
				}

				secrets, err := secretClient.List(ctx, metav1.ListOptions{})
				if err != nil {
					cobra.CompErrorln(err.Error())
					return nil, defaultFlags | cobra.ShellCompDirectiveError
				}

				secretNames := make([]string, 0, len(secrets.Items))
				for i := range secrets.Items {
					v := secrets.Items[i].GetName()
					if strings.HasPrefix(v, toComplete) {
						secretNames = append(secretNames, v)
					}
				}
				sort.Strings(secretNames)
				return secretNames, defaultFlags
			}

			secretClient, err := secretClient(kubeFactory)
			if err != nil {
				cobra.CompErrorln(err.Error())
				return nil, defaultFlags | cobra.ShellCompDirectiveError
			}
			secret, err := secretClient.Get(ctx, args[0], metav1.GetOptions{})
			if err != nil {
				cobra.CompErrorln(err.Error())
				return nil, defaultFlags | cobra.ShellCompDirectiveError
			}

			keyNames := make([]string, 0, len(secret.Data))
			for k := range secret.Data {
				if strings.HasPrefix(k, toComplete) {
					keyNames = append(keyNames, k)
				}
			}
			sort.Strings(keyNames)
			return keyNames, defaultFlags
		},
	}

	cmd.Flags().BoolVar(
		&raw, "raw", false,
		"Raw will prevent any decoding/auto-formatting of secret values",
	)
	cmd.Flags().StringVar(
		&autocompleteShellLanguage,
		"completion",
		"",
		fmt.Sprintf(
			"Generate completion script for shell (%s)",
			strings.Join(supportedShells, ", "),
		),
	)
	k8scmd.CheckErr(
		cmd.RegisterFlagCompletionFunc(
			"completion",
			func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
				shells := make([]string, 0, len(supportedShells))
				for _, shell := range supportedShells {
					if strings.HasPrefix(shell, toComplete) {
						shells = append(shells, shell)
					}
				}
				return shells, cobra.ShellCompDirectiveNoFileComp
			},
		),
	)

	cmd.SetIn(streams.In)
	cmd.SetOut(streams.Out)
	cmd.SetErr(streams.ErrOut)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.CompletionOptions = cobra.CompletionOptions{
		DisableDefaultCmd: true,
	}

	// initialize kubernetes factory (needs to install flags on the cobra
	// object so it has to happen here)
	kubectlFlags := initializeKubernetesFlags(cmd.PersistentFlags())
	kubeFactory = k8scmd.NewFactory(kubectlFlags)
	registerKubernetesAutocomplete(cmd, kubeFactory)

	return cmd, kubeFactory
}

func initializeKubernetesFlags(flags *pflag.FlagSet) *genericclioptions.ConfigFlags {
	kf := genericclioptions.
		NewConfigFlags(true).
		WithDiscoveryBurst(300).
		WithDiscoveryQPS(50.0)

	// kubectl adds a TOOOON, let's just stick with the basics for now
	// kf.AddFlags(flags)

	flags.StringVar(kf.KubeConfig, "kubeconfig", *kf.KubeConfig, "Path to the kubeconfig file to use for CLI requests.")
	flags.StringVarP(
		kf.Namespace,
		"namespace",
		"n",
		*kf.Namespace,
		"If present, the namespace scope for this CLI request",
	)

	flags.StringVar(kf.Context, "context", *kf.Context, "The name of the kubeconfig context to use")

	return kf
}

func registerKubernetesAutocomplete(cmd *cobra.Command, kubeFactory k8scmd.Factory) {
	k8scmd.CheckErr(
		cmd.RegisterFlagCompletionFunc(
			"namespace",
			func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
				return utilcomp.CompGetResource(
					kubeFactory,
					cmd,
					"namespace",
					toComplete,
				), cobra.ShellCompDirectiveNoFileComp
			},
		),
	)
	k8scmd.CheckErr(
		cmd.RegisterFlagCompletionFunc(
			"context",
			func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
				return utilcomp.ListContextsInConfig(toComplete), cobra.ShellCompDirectiveNoFileComp
			},
		),
	)
}

func Run(
	ctx context.Context,
	streams genericclioptions.IOStreams,
	secretClient v1.SecretInterface,
	opts Options,
) error {
	promptOpts := []tea.ProgramOption{
		tea.WithContext(ctx),
		tea.WithInput(streams.In),
		tea.WithOutput(streams.ErrOut),
		tea.WithoutSignalHandler(),
	}

	var secret *corev1.Secret
	if opts.SecretName == "" {
		secrets, err := secretClient.List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("fetching secrets: %v", err)
		}
		secretNames := make([]string, len(secrets.Items))
		for i := range secrets.Items {
			secretNames[i] = secrets.Items[i].GetName()
		}
		sort.Strings(secretNames)

		opts.SecretName, err = prompt.New().
			Ask("Choose secret:").
			Choose(secretNames, choose.WithTeaProgramOpts(promptOpts...))
		if err != nil {
			return fmt.Errorf("picking secret: %w", err)
		}

		for i := range secrets.Items {
			if secrets.Items[i].GetName() == opts.SecretName {
				secret = &secrets.Items[i]
				break
			}
		}
	} else {
		var err error
		secret, err = secretClient.Get(ctx, opts.SecretName, metav1.GetOptions{})
		if k8serr.IsNotFound(err) && k8sterm.IsTerminal(streams.Out) {
			msg := prompt.ThemeDefault("Choose secret:", prompt.StateError, err.Error())
			if _, err := io.Copy(streams.ErrOut, strings.NewReader(msg)); err != nil {
				return err
			}
			os.Exit(1)
		}
		if err != nil {
			return fmt.Errorf("fetching secret: %v", err)
		}
	}

	if !opts.Raw {
		switch secret.Type {
		case SecretTypeHelm:
			msg, err := FormatHelmSecret(secret)
			if err != nil {
				return err
			}
			if !strings.HasSuffix(msg, "\n") {
				msg += "\n"
			}
			if _, err := io.Copy(streams.Out, strings.NewReader(msg)); err != nil {
				return err
			}
			return nil
		}
	}

	if len(secret.Data) == 0 {
		if k8sterm.IsTerminal(streams.Out) {
			msg := prompt.ThemeDefault("Choose key:", prompt.StateError, "secret has no data")
			if _, err := io.Copy(streams.ErrOut, strings.NewReader(msg)); err != nil {
				return err
			}
			os.Exit(1)
		}
		return fmt.Errorf("secret %q has no data", secret.GetName())
	}

	if opts.SecretKey == "" {
		var keys []string
		for k := range secret.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if len(keys) == 1 {
			opts.SecretKey = keys[0]
			if k8sterm.IsTerminal(streams.Out) {
				msg := prompt.ThemeDefault("Choose key:", prompt.StateFinish, opts.SecretKey)
				if _, err := io.Copy(streams.Out, strings.NewReader(msg)); err != nil {
					return err
				}
			}
		} else {
			var err error
			opts.SecretKey, err = prompt.New().
				Ask("Choose key:").
				Choose(keys, choose.WithTeaProgramOpts(promptOpts...))
			if err != nil {
				return fmt.Errorf("picking secret key: %w", err)
			}
		}
	}

	value, ok := secret.Data[opts.SecretKey]
	if !ok {
		return fmt.Errorf("no such key in secret")
	}
	if !bytes.HasSuffix(value, []byte("\n")) && k8sterm.IsTerminal(streams.Out) {
		value = append(value, byte('\n'))
	}

	if _, err := io.Copy(streams.Out, bytes.NewReader(value)); err != nil {
		return err
	}
	return nil
}

func kubeClient(kubeFactory k8scmd.Factory) (*kubernetes.Clientset, error) {
	kubeClient, err := kubeFactory.KubernetesClientSet()
	if err != nil {
		return nil, fmt.Errorf("initializing kubernetes client: %w", err)
	}
	return kubeClient, nil
}

func secretClient(kubeFactory k8scmd.Factory) (v1.SecretInterface, error) {
	namespace, _, err := kubeFactory.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, fmt.Errorf("determining namespace: %w", err)
	}

	kubeClient, err := kubeClient(kubeFactory)
	if err != nil {
		return nil, err
	}

	secretClient := kubeClient.CoreV1().Secrets(namespace)
	return secretClient, nil
}
