package devopsanalysis

import (
	"cmp"
	"slices"
	"strings"
)

// Normalized returns a detached model with deterministic ordering and non-nil collections.
func (m Model) Normalized() Model {
	m.Tools = normalizeSlice(m.Tools)
	for index := range m.Tools {
		m.Tools[index].Evidence = normalizeEvidence(m.Tools[index].Evidence)
	}
	slices.SortFunc(m.Tools, func(left, right Tool) int {
		return strings.Compare(string(left.ID), string(right.ID))
	})

	m.ContainerBuilds = normalizeSlice(m.ContainerBuilds)
	for index := range m.ContainerBuilds {
		build := &m.ContainerBuilds[index]
		build.BaseImages = normalizeOrdered(build.BaseImages)
		build.BuildArguments = normalizeOrdered(build.BuildArguments)
		build.ExposedPorts = normalizeOrdered(build.ExposedPorts)
		build.Evidence = normalizeEvidence(build.Evidence)
	}
	slices.SortFunc(m.ContainerBuilds, compareContainerBuilds)

	m.ComposeServices = normalizeSlice(m.ComposeServices)
	for index := range m.ComposeServices {
		service := &m.ComposeServices[index]
		service.Profiles = normalizeOrdered(service.Profiles)
		service.DependsOn = normalizeOrdered(service.DependsOn)
		service.EnvironmentVariables = normalizeOrdered(service.EnvironmentVariables)
		service.Ports = normalizeOrdered(service.Ports)
		service.Evidence = normalizeEvidence(service.Evidence)
	}
	slices.SortFunc(m.ComposeServices, compareComposeServices)

	m.KubernetesResources = normalizeSlice(m.KubernetesResources)
	for index := range m.KubernetesResources {
		resource := &m.KubernetesResources[index]
		resource.Images = normalizeOrdered(resource.Images)
		resource.Ports = normalizeOrdered(resource.Ports)
		resource.Evidence = normalizeEvidence(resource.Evidence)
	}
	slices.SortFunc(m.KubernetesResources, compareKubernetesResources)

	m.HelmCharts = normalizeSlice(m.HelmCharts)
	for index := range m.HelmCharts {
		chart := &m.HelmCharts[index]
		chart.Dependencies = normalizeOrdered(chart.Dependencies)
		chart.Evidence = normalizeEvidence(chart.Evidence)
	}
	slices.SortFunc(m.HelmCharts, compareHelmCharts)

	m.TerraformBlocks = normalizeSlice(m.TerraformBlocks)
	for index := range m.TerraformBlocks {
		m.TerraformBlocks[index].Evidence = normalizeEvidence(m.TerraformBlocks[index].Evidence)
	}
	slices.SortFunc(m.TerraformBlocks, compareTerraformBlocks)

	m.Pipelines = normalizeSlice(m.Pipelines)
	for index := range m.Pipelines {
		pipeline := &m.Pipelines[index]
		pipeline.Triggers = normalizeOrdered(pipeline.Triggers)
		pipeline.Jobs = normalizeSlice(pipeline.Jobs)
		for jobIndex := range pipeline.Jobs {
			job := &pipeline.Jobs[jobIndex]
			job.Needs = normalizeOrdered(job.Needs)
			job.Evidence = normalizeEvidence(job.Evidence)
		}
		slices.SortFunc(pipeline.Jobs, comparePipelineJobs)
		pipeline.Evidence = normalizeEvidence(pipeline.Evidence)
	}
	slices.SortFunc(m.Pipelines, comparePipelines)

	m.GitOpsResources = normalizeSlice(m.GitOpsResources)
	for index := range m.GitOpsResources {
		m.GitOpsResources[index].Evidence = normalizeEvidence(m.GitOpsResources[index].Evidence)
	}
	slices.SortFunc(m.GitOpsResources, compareGitOpsResources)

	m.ObservabilityResources = normalizeSlice(m.ObservabilityResources)
	for index := range m.ObservabilityResources {
		resource := &m.ObservabilityResources[index]
		resource.Signals = normalizeOrdered(resource.Signals)
		resource.Evidence = normalizeEvidence(resource.Evidence)
	}
	slices.SortFunc(m.ObservabilityResources, compareObservabilityResources)

	m.SecurityControls = normalizeSlice(m.SecurityControls)
	for index := range m.SecurityControls {
		m.SecurityControls[index].Evidence = normalizeEvidence(m.SecurityControls[index].Evidence)
	}
	slices.SortFunc(m.SecurityControls, compareSecurityControls)

	m.Diagnostics = normalizeSlice(m.Diagnostics)
	slices.SortFunc(m.Diagnostics, compareDiagnostics)
	return m
}

func normalizeEvidence(evidence []Evidence) []Evidence {
	evidence = normalizeSlice(evidence)
	slices.SortFunc(evidence, compareEvidence)
	return slices.Compact(evidence)
}

func normalizeOrdered[S ~[]E, E cmp.Ordered](entries S) S {
	entries = normalizeSlice(entries)
	slices.Sort(entries)
	return slices.Compact(entries)
}

func normalizeSlice[S ~[]E, E any](entries S) S {
	normalized := slices.Clone(entries)
	if normalized == nil {
		return make(S, 0)
	}
	return normalized
}

func compareContainerBuilds(left, right ContainerBuild) int {
	return strings.Compare(left.ID, right.ID)
}

func compareComposeServices(left, right ComposeService) int {
	return strings.Compare(left.ID, right.ID)
}

func compareKubernetesResources(left, right KubernetesResource) int {
	return strings.Compare(left.ID, right.ID)
}

func compareHelmCharts(left, right HelmChart) int {
	return strings.Compare(left.ID, right.ID)
}

func compareTerraformBlocks(left, right TerraformBlock) int {
	return strings.Compare(left.ID, right.ID)
}

func comparePipelines(left, right Pipeline) int {
	return strings.Compare(left.ID, right.ID)
}

func comparePipelineJobs(left, right PipelineJob) int {
	return strings.Compare(left.ID, right.ID)
}

func compareGitOpsResources(left, right GitOpsResource) int {
	return strings.Compare(left.ID, right.ID)
}

func compareObservabilityResources(left, right ObservabilityResource) int {
	return strings.Compare(left.ID, right.ID)
}

func compareSecurityControls(left, right SecurityControl) int {
	return strings.Compare(left.ID, right.ID)
}

func compareEvidence(left, right Evidence) int {
	if compared := strings.Compare(left.Path, right.Path); compared != 0 {
		return compared
	}
	if compared := strings.Compare(string(left.Kind), string(right.Kind)); compared != 0 {
		return compared
	}
	if compared := cmp.Compare(left.StartLine, right.StartLine); compared != 0 {
		return compared
	}
	if compared := cmp.Compare(left.StartColumn, right.StartColumn); compared != 0 {
		return compared
	}
	if compared := cmp.Compare(left.EndLine, right.EndLine); compared != 0 {
		return compared
	}
	return cmp.Compare(left.EndColumn, right.EndColumn)
}

func compareDiagnostics(left, right Diagnostic) int {
	if compared := strings.Compare(left.Path, right.Path); compared != 0 {
		return compared
	}
	if compared := strings.Compare(left.Code, right.Code); compared != 0 {
		return compared
	}
	if compared := strings.Compare(left.Message, right.Message); compared != 0 {
		return compared
	}
	return strings.Compare(string(left.Level), string(right.Level))
}
