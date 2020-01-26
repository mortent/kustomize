package cmd

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/kustomize/kstatus/observe"
	"sigs.k8s.io/kustomize/kstatus/observe/collector"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"time"
)

func GetTreeRunner() *TreeRunner {
	r := &TreeRunner{
		newResolverFunc: newResolver,
	}
	c := &cobra.Command{
		Use:     "tree DIR...",
		RunE:    r.runE,
	}
	c.Flags().BoolVar(&r.IncludeSubpackages, "include-subpackages", true,
		"also print resources from subpackages.")
	c.Flags().DurationVar(&r.Interval, "interval", 2*time.Second,
		"check every n seconds. Default is every 2 seconds.")
	c.Flags().DurationVar(&r.Timeout, "timeout", 60*time.Second,
		"give up after n seconds. Default is 60 seconds.")
	c.Flags().BoolVar(&r.StopOnComplete, "stop-on-complete", true,
		"exit when all resources have fully reconciled")

	r.Command = c
	return r
}

func TreeCommand() *cobra.Command {
	return GetTreeRunner().Command
}

// WaitRunner captures the parameters for the command and contains
// the run function.
type TreeRunner struct {
	IncludeSubpackages bool
	Interval           time.Duration
	Timeout            time.Duration
	StopOnComplete     bool
	Command            *cobra.Command

	newResolverFunc newResolverFunc
}

// runE implements the logic of the command and will call the Wait command in the wait
// package, use a ResourceStatusCollector to capture the events from the channel, and the
// TablePrinter to display the information.
func (r *TreeRunner) runE(c *cobra.Command, args []string) error {
	ctx := context.Background()

	config := ctrl.GetConfigOrDie()
	mapper, err := apiutil.NewDiscoveryRESTMapper(config)
	if err != nil {
		return err
	}

	k8sClient, err := client.New(config, client.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		return err
	}

	observer := observe.NewStatusObserver(k8sClient, mapper)

	captureFilter := &CaptureIdentifiersFilter{
		Mapper: mapper,
	}
	filters := []kio.Filter{captureFilter}

	var inputs []kio.Reader
	for _, a := range args {
		inputs = append(inputs, kio.LocalPackageReader{
			PackagePath:        a,
			IncludeSubpackages: r.IncludeSubpackages,
		})
	}
	if len(inputs) == 0 {
		inputs = append(inputs, &kio.ByteReader{Reader: c.InOrStdin()})
	}

	err = kio.Pipeline{
		Inputs:  inputs,
		Filters: filters,
	}.Execute()
	if err != nil {
		return errors.WrapPrefix(err, "error reading manifests", 1)
	}

	coll := collector.NewObservedStatusCollector(captureFilter.Identifiers)
	stop := make(chan struct{})
	printer := NewTreePrinter(coll, c.OutOrStdout())
	printingFinished := printer.PrintUntil(stop, 1 * time.Second)

	eventChannel := observer.Observe(ctx, captureFilter.Identifiers, r.StopOnComplete)
	completed := coll.Observe(eventChannel, stop)

	<-completed
	close(stop)
	<-printingFinished
	return nil
}
