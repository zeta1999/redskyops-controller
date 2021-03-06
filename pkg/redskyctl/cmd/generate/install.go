/*
Copyright 2019 GramLabs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package generate

import (
	"fmt"

	"github.com/redskyops/redskyops-controller/internal/setup"
	cmdutil "github.com/redskyops/redskyops-controller/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

const (
	generateInstallLong    = `Generate installation manifests for Red Sky Ops`
	generateInstallExample = ``
)

// TODO What about the namespace the pod will execute in? Currently we use the default namespace of the current context (hopefully "default")

type GenerateInstallOptions struct {
	Kubectl   *cmdutil.Kubectl
	Namespace string

	cmdutil.IOStreams
}

func NewGenerateInstallOptions(ioStreams cmdutil.IOStreams) *GenerateInstallOptions {
	return &GenerateInstallOptions{
		Namespace: "redsky-system",
		Kubectl:   cmdutil.NewKubectl(),
		IOStreams: ioStreams,
	}
}

func NewGenerateInstallCmd(ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewGenerateInstallOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "install",
		Short:   "Generate Red Sky Ops manifests",
		Long:    generateInstallLong,
		Example: generateInstallExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(cmd, o.Complete())
			cmdutil.CheckErr(cmd, o.Run())
		},
	}

	// This won't show up in the help, but it will still get populated
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "The namespace to be used by the manager.")

	o.Kubectl.AddFlags(cmd)

	return cmd
}

func (o *GenerateInstallOptions) Complete() error {
	if err := o.Kubectl.Complete(); err != nil {
		return err
	}
	return nil
}

func (o *GenerateInstallOptions) Run() error {
	// Create an argument list to generate the installation manifests
	args := []string{"run", "redsky-bootstrap"}

	// Create a single attached pod
	args = append(args, "--restart", "Never", "--attach")

	// Quietly remove the pod when we are done
	args = append(args, "--rm", "--quiet")

	// Use the image embedded in the code
	args = append(args, "--image", setup.Image)
	// TODO We may need to overwrite this for offline clusters
	args = append(args, "--image-pull-policy", setup.ImagePullPolicy)

	// Do not allow the pod to access the API
	args = append(args, "--overrides", `{"spec":{"automountServiceAccountToken":false}}`)

	// Overwrite the "redsky-system" namespace if configured
	if o.Namespace != "" {
		args = append(args, "--env", fmt.Sprintf("NAMESPACE=%s", o.Namespace))
	}

	// Arguments passed to the container
	args = append(args, "--", "install")

	// Run it
	cmd := o.Kubectl.NewCmd(args...)
	cmd.Stdout = o.Out
	cmd.Stderr = o.ErrOut
	return cmd.Run()
}
