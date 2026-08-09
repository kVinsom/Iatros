package analysis

import (
	"context"
	"errors"

	"github.com/kVinsom/Iatros/internal/manifest"
	"github.com/kVinsom/Iatros/internal/project"
	"github.com/kVinsom/Iatros/internal/topology"
)

var errLocalTopologyAnalyzerUnavailable = errors.New("local topology analyzer is unavailable")

type topologyDiscovery interface {
	Discover(context.Context, string) (Inventory, error)
}

type projectBoundaryDetector interface {
	Detect(context.Context, project.Snapshot) (project.Model, error)
}

type manifestFactAnalyzer interface {
	Analyze(context.Context, manifest.Source, manifest.Snapshot) (manifest.Result, error)
}

type topologyModelBuilder interface {
	Build(context.Context, topology.Snapshot) (topology.Model, error)
}

// LocalTopologyAnalyzer composes bounded discovery, manifest parsing, and topology association.
type LocalTopologyAnalyzer struct {
	discovery topologyDiscovery
	projects  projectBoundaryDetector
	manifests manifestFactAnalyzer
	topology  topologyModelBuilder
	profile   ScalingProfileName
}

// NewLocalTopologyAnalyzer creates the local topology pipeline with conservative profiles.
func NewLocalTopologyAnalyzer() (LocalTopologyAnalyzer, error) {
	return NewLocalTopologyAnalyzerWithProfile(SmallScalingProfile())
}

// NewLocalTopologyAnalyzerWithProfile creates the topology pipeline with one validated profile.
func NewLocalTopologyAnalyzerWithProfile(
	profile ScalingProfile,
) (LocalTopologyAnalyzer, error) {
	if err := profile.Validate(); err != nil {
		return LocalTopologyAnalyzer{}, err
	}
	discovery, err := NewLocalDiscovery(profile.Discovery)
	if err != nil {
		return LocalTopologyAnalyzer{}, err
	}
	projects, err := project.NewDetector(profile.Project)
	if err != nil {
		return LocalTopologyAnalyzer{}, err
	}
	manifests, err := manifest.NewDefaultAnalyzer(profile.Manifest)
	if err != nil {
		return LocalTopologyAnalyzer{}, err
	}
	topologyBuilder, err := topology.NewBuilder(profile.Topology)
	if err != nil {
		return LocalTopologyAnalyzer{}, err
	}
	return LocalTopologyAnalyzer{
		discovery: discovery,
		projects:  projects,
		manifests: manifests,
		topology:  topologyBuilder,
		profile:   profile.Name,
	}, nil
}

// Analyze builds local repository topology without executing code, using the network, or writing files.
func (a LocalTopologyAnalyzer) Analyze(ctx context.Context, request Request) (topology.Model, error) {
	profile := normalizedScalingProfileName(a.profile)
	requestedProfile := request.Profile
	if requestedProfile == "" {
		requestedProfile = profile
	}
	if requestedProfile != profile {
		return topology.Model{}, ErrScalingProfileUnavailable
	}
	if err := ctx.Err(); err != nil {
		return topology.Model{}, err
	}
	if a.discovery == nil || a.projects == nil || a.manifests == nil || a.topology == nil {
		return topology.Model{}, errLocalTopologyAnalyzerUnavailable
	}

	inventory, err := a.discovery.Discover(ctx, request.Root)
	if err != nil {
		return topology.Model{}, err
	}
	if err := ctx.Err(); err != nil {
		return topology.Model{}, err
	}
	projects, err := a.projects.Detect(ctx, project.Snapshot{
		Files: inventory.Files, Partial: inventory.Partial,
	})
	if err != nil {
		return topology.Model{}, err
	}
	if err := ctx.Err(); err != nil {
		return topology.Model{}, err
	}

	source, err := NewLocalManifestSource(request.Root)
	if err != nil {
		return topology.Model{}, err
	}
	manifests, analyzeErr := a.manifests.Analyze(ctx, source, manifest.Snapshot{
		Files: inventory.Files, Partial: inventory.Partial,
	})
	closeErr := source.Close()
	if analyzeErr != nil || closeErr != nil {
		return topology.Model{}, errors.Join(analyzeErr, closeErr)
	}
	if err := ctx.Err(); err != nil {
		return topology.Model{}, err
	}

	model, err := a.topology.Build(ctx, topology.Snapshot{
		Projects:           projects,
		Manifests:          manifests,
		NestedRepositories: inventory.NestedRepositories,
		Issues:             topologyIssuesFromDiscovery(inventory.Issues),
	})
	if err != nil {
		return topology.Model{}, err
	}
	if err := ctx.Err(); err != nil {
		return topology.Model{}, err
	}
	return model, nil
}

func topologyIssuesFromDiscovery(discoveryIssues []DiscoveryIssue) []topology.Issue {
	issues := make([]topology.Issue, 0, len(discoveryIssues))
	for _, discoveryIssue := range discoveryIssues {
		issues = append(issues, topology.Issue{
			Code:    discoveryIssue.Code,
			Path:    discoveryIssue.Path,
			Message: discoveryIssue.Message,
		})
	}
	return issues
}
