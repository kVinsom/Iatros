package codeanalysis

import (
	"slices"
	"strings"
)

// Normalized returns a detached model with deterministic ordering and non-nil collections.
func (m Model) Normalized() Model {
	m.Services = normalizeSlice(m.Services)
	for index := range m.Services {
		service := &m.Services[index]
		service.Entrypoints = normalizeOrderedStrings(service.Entrypoints)
		service.Evidence = normalizeEvidence(service.Evidence)
	}
	slices.SortFunc(m.Services, compareServices)

	m.Frameworks = normalizeSlice(m.Frameworks)
	for index := range m.Frameworks {
		m.Frameworks[index].Evidence = normalizeEvidence(m.Frameworks[index].Evidence)
	}
	slices.SortFunc(m.Frameworks, compareFrameworks)

	m.PortBindings = normalizeSlice(m.PortBindings)
	for index := range m.PortBindings {
		m.PortBindings[index].Evidence = normalizeEvidence(m.PortBindings[index].Evidence)
	}
	slices.SortFunc(m.PortBindings, comparePortBindings)

	m.APIEndpoints = normalizeSlice(m.APIEndpoints)
	for index := range m.APIEndpoints {
		m.APIEndpoints[index].Evidence = normalizeEvidence(m.APIEndpoints[index].Evidence)
	}
	slices.SortFunc(m.APIEndpoints, compareAPIEndpoints)

	m.EnvironmentVariables = normalizeSlice(m.EnvironmentVariables)
	for index := range m.EnvironmentVariables {
		m.EnvironmentVariables[index].Evidence = normalizeEvidence(m.EnvironmentVariables[index].Evidence)
	}
	slices.SortFunc(m.EnvironmentVariables, compareEnvironmentVariables)

	m.ResourceDependencies = normalizeSlice(m.ResourceDependencies)
	for index := range m.ResourceDependencies {
		m.ResourceDependencies[index].Evidence = normalizeEvidence(m.ResourceDependencies[index].Evidence)
	}
	slices.SortFunc(m.ResourceDependencies, compareResourceDependencies)

	m.Diagnostics = normalizeSlice(m.Diagnostics)
	slices.SortFunc(m.Diagnostics, compareDiagnostics)
	return m
}

func normalizeEvidence(evidence []Evidence) []Evidence {
	evidence = normalizeSlice(evidence)
	slices.SortFunc(evidence, compareEvidence)
	return slices.Compact(evidence)
}

func normalizeOrderedStrings(entries []string) []string {
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

func compareServices(left, right Service) int {
	return strings.Compare(string(left.ID), string(right.ID))
}

func compareFrameworks(left, right Framework) int {
	if compared := strings.Compare(string(left.ServiceID), string(right.ServiceID)); compared != 0 {
		return compared
	}
	return strings.Compare(left.ID, right.ID)
}

func comparePortBindings(left, right PortBinding) int {
	for _, field := range [][2]string{
		{string(left.ServiceID), string(right.ServiceID)},
		{string(left.Protocol), string(right.Protocol)},
		{left.Name, right.Name},
		{left.Reference, right.Reference},
		{string(left.ReferenceKind), string(right.ReferenceKind)},
	} {
		if compared := strings.Compare(field[0], field[1]); compared != 0 {
			return compared
		}
	}
	return compareIntegers(int(left.Port), int(right.Port))
}

func compareAPIEndpoints(left, right APIEndpoint) int {
	for _, field := range [][2]string{
		{string(left.ServiceID), string(right.ServiceID)},
		{string(left.Protocol), string(right.Protocol)},
		{left.Path, right.Path},
		{left.Method, right.Method},
		{left.Operation, right.Operation},
	} {
		if compared := strings.Compare(field[0], field[1]); compared != 0 {
			return compared
		}
	}
	return 0
}

func compareEnvironmentVariables(left, right EnvironmentVariable) int {
	if compared := strings.Compare(string(left.ServiceID), string(right.ServiceID)); compared != 0 {
		return compared
	}
	return strings.Compare(left.Name, right.Name)
}

func compareResourceDependencies(left, right ResourceDependency) int {
	for _, field := range [][2]string{
		{string(left.ServiceID), string(right.ServiceID)},
		{string(left.Kind), string(right.Kind)},
		{left.Technology, right.Technology},
		{left.Name, right.Name},
	} {
		if compared := strings.Compare(field[0], field[1]); compared != 0 {
			return compared
		}
	}
	return 0
}

func compareEvidence(left, right Evidence) int {
	for _, field := range [][2]string{
		{left.Path, right.Path},
		{string(left.Kind), string(right.Kind)},
	} {
		if compared := strings.Compare(field[0], field[1]); compared != 0 {
			return compared
		}
	}
	for _, coordinate := range [][2]int{
		{left.StartLine, right.StartLine},
		{left.StartColumn, right.StartColumn},
		{left.EndLine, right.EndLine},
		{left.EndColumn, right.EndColumn},
	} {
		if compared := compareIntegers(coordinate[0], coordinate[1]); compared != 0 {
			return compared
		}
	}
	return 0
}

func compareIntegers(left, right int) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func compareDiagnostics(left, right Diagnostic) int {
	for _, field := range [][2]string{
		{left.Path, right.Path},
		{left.Code, right.Code},
		{left.Message, right.Message},
		{string(left.Level), string(right.Level)},
	} {
		if compared := strings.Compare(field[0], field[1]); compared != 0 {
			return compared
		}
	}
	return 0
}
