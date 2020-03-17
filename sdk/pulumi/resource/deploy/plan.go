// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deploy

import (
	"context"
	"math"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"

	"github.com/pulumi/pulumi/sdk/pulumi/diag"
	"github.com/pulumi/pulumi/pkg/util/result"
	"github.com/pulumi/pulumi/sdk/pulumi/resource"
	"github.com/pulumi/pulumi/sdk/pulumi/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/pulumi/resource/graph"
	"github.com/pulumi/pulumi/sdk/pulumi/resource/plugin"
	"github.com/pulumi/pulumi/sdk/pulumi/tokens"
	"github.com/pulumi/pulumi/sdk/pulumi/util/contract"
)

// BackendClient provides an interface for retrieving information about other stacks.
type BackendClient interface {
	// GetStackOutputs returns the outputs (if any) for the named stack or an error if the stack cannot be found.
	GetStackOutputs(ctx context.Context, name string) (resource.PropertyMap, error)

	// GetStackResourceOutputs returns the resource outputs for a stack, or an error if the stack
	// cannot be found. Resources are retrieved from the latest stack snapshot, which may include
	// ongoing updates. They are returned in a `PropertyMap` mapping resource URN to another
	// `Propertymap` with members `type` (containing the Pulumi type ID for the resource) and
	// `outputs` (containing the resource outputs themselves).
	GetStackResourceOutputs(ctx context.Context, stackName string) (resource.PropertyMap, error)
}

// Options controls the planning and deployment process.
type Options struct {
	Events            Events         // an optional events callback interface.
	Parallel          int            // the degree of parallelism for resource operations (<=1 for serial).
	Refresh           bool           // whether or not to refresh before executing the plan.
	RefreshOnly       bool           // whether or not to exit after refreshing.
	RefreshTargets    []resource.URN // The specific resources to refresh during a refresh op.
	ReplaceTargets    []resource.URN // Specific resources to replace.
	DestroyTargets    []resource.URN // Specific resources to destroy.
	UpdateTargets     []resource.URN // Specific resources to update.
	TargetDependents  bool           // true if we're allowing things to proceed, even with unspecified targets
	TrustDependencies bool           // whether or not to trust the resource dependency graph.
	UseLegacyDiff     bool           // whether or not to use legacy diffing behavior.
}

// DegreeOfParallelism returns the degree of parallelism that should be used during the
// planning and deployment process.
func (o Options) DegreeOfParallelism() int {
	if o.Parallel <= 1 {
		return 1
	}
	return o.Parallel
}

// InfiniteParallelism returns whether or not the requested level of parallelism is unbounded.
func (o Options) InfiniteParallelism() bool {
	return o.Parallel == math.MaxInt32
}

// StepExecutorEvents is an interface that can be used to hook resource lifecycle events.
type StepExecutorEvents interface {
	OnResourceStepPre(step Step) (interface{}, error)
	OnResourceStepPost(ctx interface{}, step Step, status resource.Status, err error) error
	OnResourceOutputs(step Step) error
}

// PolicyEvents is an interface that can be used to hook policy violation events.
type PolicyEvents interface {
	OnPolicyViolation(resource.URN, plugin.AnalyzeDiagnostic)
}

// Events is an interface that can be used to hook interesting engine/planning events.
type Events interface {
	StepExecutorEvents
	PolicyEvents
}

// PlanPendingOperationsError is an error returned from `NewPlan` if there exist pending operations in the
// snapshot that we are preparing to operate upon. The engine does not allow any operations to be pending
// when operating on a snapshot.
type PlanPendingOperationsError struct {
	Operations []resource.Operation
}

func (p PlanPendingOperationsError) Error() string {
	return "one or more operations are currently pending"
}

// Plan is the output of analyzing resource graphs and contains the steps necessary to perform an infrastructure
// deployment.  A plan can be generated out of whole cloth from a resource graph -- in the case of new deployments --
// however, it can alternatively be generated by diffing two resource graphs -- in the case of updates to existing
// stacks (presumably more common).  The plan contains step objects that can be used to drive a deployment.
type Plan struct {
	ctx                  *plugin.Context                  // the plugin context (for provider operations).
	target               *Target                          // the deployment target.
	prev                 *Snapshot                        // the old resource snapshot for comparison.
	olds                 map[resource.URN]*resource.State // a map of all old resources.
	source               Source                           // the source of new resources.
	localPolicyPackPaths []string                         // the policy packs to run during this plan's generation.
	preview              bool                             // true if this plan is to be previewed rather than applied.
	depGraph             *graph.DependencyGraph           // the dependency graph of the old snapshot
	providers            *providers.Registry              // the provider registry for this plan.
}

// addDefaultProviders adds any necessary default provider definitions and references to the given snapshot. Version
// information for these providers is sourced from the snapshot's manifest; inputs parameters are sourced from the
// stack's configuration.
func addDefaultProviders(target *Target, source Source, prev *Snapshot) error {
	if prev == nil {
		return nil
	}

	// Pull the versions we'll use for default providers from the snapshot's manifest.
	defaultProviderVersions := make(map[tokens.Package]*semver.Version)
	for _, p := range prev.Manifest.Plugins {
		defaultProviderVersions[tokens.Package(p.Name)] = p.Version
	}

	// Determine the necessary set of default providers and inject references to default providers as appropriate.
	//
	// We do this by scraping the snapshot for custom resources that does not reference a provider and adding
	// default providers for these resources' packages. Each of these resources is rewritten to reference the default
	// provider for its package.
	//
	// The configuration for each default provider is pulled from the stack's configuration information.
	var defaultProviders []*resource.State
	defaultProviderRefs := make(map[tokens.Package]providers.Reference)
	for _, res := range prev.Resources {
		if providers.IsProviderType(res.URN.Type()) || !res.Custom || res.Provider != "" {
			continue
		}

		pkg := res.URN.Type().Package()
		ref, ok := defaultProviderRefs[pkg]
		if !ok {
			cfg, err := target.GetPackageConfig(pkg)
			if err != nil {
				return errors.Errorf("could not fetch configuration for default provider '%v'", pkg)
			}

			inputs := make(resource.PropertyMap)
			for k, v := range cfg {
				inputs[resource.PropertyKey(k.Name())] = resource.NewStringProperty(v)
			}
			if version, ok := defaultProviderVersions[pkg]; ok {
				inputs["version"] = resource.NewStringProperty(version.String())
			}

			urn, id := defaultProviderURN(target, source, pkg), resource.ID(uuid.NewV4().String())
			ref, err = providers.NewReference(urn, id)
			contract.Assert(err == nil)

			provider := &resource.State{
				Type:    urn.Type(),
				URN:     urn,
				Custom:  true,
				ID:      id,
				Inputs:  inputs,
				Outputs: inputs,
			}
			defaultProviders = append(defaultProviders, provider)
			defaultProviderRefs[pkg] = ref
		}
		res.Provider = ref.String()
	}

	// If any default providers are necessary, prepend their definitions to the snapshot's resources. This trivially
	// guarantees that all default provider references name providers that precede the referent in the snapshot.
	if len(defaultProviders) != 0 {
		prev.Resources = append(defaultProviders, prev.Resources...)
	}

	return nil
}

// NewPlan creates a new deployment plan from a resource snapshot plus a package to evaluate.
//
// From the old and new states, it understands how to orchestrate an evaluation and analyze the resulting resources.
// The plan may be used to simply inspect a series of operations, or actually perform them; these operations are
// generated based on analysis of the old and new states.  If a resource exists in new, but not old, for example, it
// results in a create; if it exists in both, but is different, it results in an update; and so on and so forth.
//
// Note that a plan uses internal concurrency and parallelism in various ways, so it must be closed if for some reason
// a plan isn't carried out to its final conclusion.  This will result in cancelation and reclamation of OS resources.
func NewPlan(ctx *plugin.Context, target *Target, prev *Snapshot, source Source,
	localPolicyPackPaths []string, preview bool, backendClient BackendClient) (*Plan, error) {

	contract.Assert(ctx != nil)
	contract.Assert(target != nil)
	contract.Assert(source != nil)

	// Add any necessary default provider references to the previous snapshot in order to accommodate stacks that were
	// created prior to the changes that added first-class providers. We do this here rather than in the migration
	// package s.t. the inputs to any default providers (which we fetch from the stacks's configuration) are as
	// accurate as possible.
	if err := addDefaultProviders(target, source, prev); err != nil {
		return nil, err
	}

	// Migrate provider resources from the old, output-less format to the new format where all inputs are reflected as
	// outputs.
	if prev != nil {
		for _, res := range prev.Resources {
			// If we have no old outputs for a provider, use its old inputs as its old outputs. This handles the
			// scenario where the CLI is being upgraded from a version that did not reflect provider inputs to
			// provider outputs, and a provider is being upgraded from a version that did not implement DiffConfig to
			// a version that does.
			if providers.IsProviderType(res.URN.Type()) && len(res.Inputs) != 0 && len(res.Outputs) == 0 {
				res.Outputs = res.Inputs
			}
		}
	}

	var depGraph *graph.DependencyGraph
	var oldResources []*resource.State

	// Produce a map of all old resources for fast resources.
	//
	// NOTE: we can and do mutate prev.Resources, olds, and depGraph during execution after performing a refresh. See
	// planExecutor.refresh for details.
	olds := make(map[resource.URN]*resource.State)
	if prev != nil {
		if prev.PendingOperations != nil && !preview {
			return nil, PlanPendingOperationsError{prev.PendingOperations}
		}
		oldResources = prev.Resources

		for _, oldres := range oldResources {
			// Ignore resources that are pending deletion; these should not be recorded in the LUT.
			if oldres.Delete {
				continue
			}

			urn := oldres.URN
			if olds[urn] != nil {
				return nil, errors.Errorf("unexpected duplicate resource '%s'", urn)
			}
			olds[urn] = oldres
		}

		depGraph = graph.NewDependencyGraph(oldResources)
	}

	// Create a new builtin provider. This provider implements features such as `getStack`.
	builtins := newBuiltinProvider(backendClient)

	// Create a new provider registry. Although we really only need to pass in any providers that were present in the
	// old resource list, the registry itself will filter out other sorts of resources when processing the prior state,
	// so we just pass all of the old resources.
	reg, err := providers.NewRegistry(ctx.Host, oldResources, preview, builtins)
	if err != nil {
		return nil, err
	}

	return &Plan{
		ctx:                  ctx,
		target:               target,
		prev:                 prev,
		olds:                 olds,
		source:               source,
		localPolicyPackPaths: localPolicyPackPaths,
		preview:              preview,
		depGraph:             depGraph,
		providers:            reg,
	}, nil
}

func (p *Plan) Ctx() *plugin.Context                   { return p.ctx }
func (p *Plan) Target() *Target                        { return p.target }
func (p *Plan) Diag() diag.Sink                        { return p.ctx.Diag }
func (p *Plan) Prev() *Snapshot                        { return p.prev }
func (p *Plan) Olds() map[resource.URN]*resource.State { return p.olds }
func (p *Plan) Source() Source                         { return p.source }

func (p *Plan) GetProvider(ref providers.Reference) (plugin.Provider, bool) {
	return p.providers.GetProvider(ref)
}

// generateURN generates a resource's URN from its parent, type, and name under the scope of the plan's stack and
// project.
func (p *Plan) generateURN(parent resource.URN, ty tokens.Type, name tokens.QName) resource.URN {
	// Use the resource goal state name to produce a globally unique URN.
	parentType := tokens.Type("")
	if parent != "" && parent.Type() != resource.RootStackType {
		// Skip empty parents and don't use the root stack type; otherwise, use the full qualified type.
		parentType = parent.QualifiedType()
	}

	return resource.NewURN(p.Target().Name, p.source.Project(), parentType, ty, name)
}

// defaultProviderURN generates the URN for the global provider given a package.
func defaultProviderURN(target *Target, source Source, pkg tokens.Package) resource.URN {
	return resource.NewURN(target.Name, source.Project(), "", providers.MakeProviderType(pkg), "default")
}

// generateEventURN generates a URN for the resource associated with the given event.
func (p *Plan) generateEventURN(event SourceEvent) resource.URN {
	contract.Require(event != nil, "event != nil")

	switch e := event.(type) {
	case RegisterResourceEvent:
		goal := e.Goal()
		return p.generateURN(goal.Parent, goal.Type, goal.Name)
	case ReadResourceEvent:
		return p.generateURN(e.Parent(), e.Type(), e.Name())
	case RegisterResourceOutputsEvent:
		return e.URN()
	default:
		return ""
	}
}

// Execute executes a plan to completion, using the given cancellation context and running a preview
// or update.
func (p *Plan) Execute(ctx context.Context, opts Options, preview bool) result.Result {
	planExec := &planExecutor{plan: p}
	return planExec.Execute(ctx, opts, preview)
}
