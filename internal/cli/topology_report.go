package cli

import (
	"io"

	"github.com/kVinsom/Iatros/internal/analysis"
)

func writeTopologyReport(
	writer io.Writer,
	format string,
	report analysis.TopologyReport,
) error {
	if format == formatJSON {
		if topologyReportNeedsNormalization(report) {
			report = report.Normalized()
		}
		return writeJSON(writer, report)
	}
	return writeTopologyTextReport(writer, report)
}

func topologyReportNeedsNormalization(report analysis.TopologyReport) bool {
	if report.Projects == nil || report.Workspaces == nil || report.Dependencies == nil ||
		report.Diagnostics == nil {
		return true
	}
	for _, project := range report.Projects {
		if project.WorkspaceRoots == nil || topologyBoundaryNeedsNormalization(
			project.Markers,
			project.Components,
		) {
			return true
		}
	}
	for _, workspace := range report.Workspaces {
		if topologyBoundaryNeedsNormalization(workspace.Markers, workspace.Components) ||
			workspace.ContainedProjects == nil || workspace.DeclaredProjects == nil ||
			workspace.ExcludedProjects == nil || workspace.Declarations == nil {
			return true
		}
		for _, declaration := range workspace.Declarations {
			if declaration.ProjectRoots == nil {
				return true
			}
		}
	}
	for _, dependency := range report.Dependencies {
		if dependency.TargetProjects == nil {
			return true
		}
	}
	return false
}

func topologyBoundaryNeedsNormalization(
	markers []analysis.TopologyMarker,
	components []analysis.TopologyComponent,
) bool {
	if markers == nil || components == nil {
		return true
	}
	for _, marker := range markers {
		if marker.Evidence == nil {
			return true
		}
	}
	for _, component := range components {
		if component.Constraints == nil {
			return true
		}
	}
	return false
}

func writeTopologyTextReport(writer io.Writer, report analysis.TopologyReport) error {
	output := newTextWriter(writer)
	output.line("IATROS Repository Topology")
	output.printf("Schema version: %s\n", report.SchemaVersion)
	output.printf("Report type: %s\n", report.ReportType)
	output.printf("Status: %s\n", textStatus(report.Status))
	output.printf("Target kind: %s\n", report.Target.Kind)
	output.printf("Target: %s\n", report.Target.Path)

	if report.Status == analysis.StatusFailed {
		output.line()
		output.line("No topology was produced.")
		writeTextDiagnostics(output, report.Diagnostics)
		return output.flush()
	}

	writeTopologyTextSummary(output, report.Summary)
	writeTopologyTextProjects(output, report.Projects)
	writeTopologyTextWorkspaces(output, report.Workspaces)
	writeTopologyTextDependencies(output, report.Dependencies)
	writeTextDiagnostics(output, report.Diagnostics)
	return output.flush()
}

func writeTopologyTextSummary(output *textWriter, summary analysis.TopologySummary) {
	output.line()
	output.line("Summary:")
	output.printf("- Projects total: %d\n", summary.ProjectsTotal)
	output.printf("- Workspaces total: %d\n", summary.WorkspacesTotal)
	output.printf("- Components total: %d\n", summary.ComponentsTotal)
	output.printf("- Dependencies total: %d\n", summary.DependenciesTotal)
	output.printf("- Internal dependencies: %d\n", summary.InternalDependencies)
	output.printf("- Unresolved dependencies: %d\n", summary.UnresolvedDependencies)
	output.printf("- Ambiguous dependencies: %d\n", summary.AmbiguousDependencies)
}

func writeTopologyTextProjects(output *textWriter, projects []analysis.TopologyProject) {
	output.line()
	output.line("Projects:")
	if len(projects) == 0 {
		output.line("- None")
		return
	}
	for _, project := range projects {
		output.printf("- Root: %s\n", project.Root)
		output.printf("  Kind: %s\n", project.Kind)
		output.printf("  Primary workspace root: %s\n", optionalText(project.PrimaryWorkspaceRoot))
		writeTopologyStringList(output, "  Workspace roots", "    ", project.WorkspaceRoots)
		writeTopologyMarkers(output, "  ", project.Markers)
		writeTopologyComponents(output, "  ", project.Components)
	}
}

func writeTopologyTextWorkspaces(output *textWriter, workspaces []analysis.TopologyWorkspace) {
	output.line()
	output.line("Workspaces:")
	if len(workspaces) == 0 {
		output.line("- None")
		return
	}
	for _, workspace := range workspaces {
		output.printf("- Root: %s\n", workspace.Root)
		writeTopologyMarkers(output, "  ", workspace.Markers)
		writeTopologyComponents(output, "  ", workspace.Components)
		writeTopologyStringList(output, "  Contained projects", "    ", workspace.ContainedProjects)
		writeTopologyStringList(output, "  Declared projects", "    ", workspace.DeclaredProjects)
		writeTopologyStringList(output, "  Excluded projects", "    ", workspace.ExcludedProjects)
		writeTopologyDeclarations(output, workspace.Declarations)
	}
}

func writeTopologyTextDependencies(
	output *textWriter,
	dependencies []analysis.TopologyDependency,
) {
	output.line()
	output.line("Dependencies:")
	if len(dependencies) == 0 {
		output.line("- None")
		return
	}
	for _, dependency := range dependencies {
		output.printf("- Name: %s\n", dependency.Name)
		output.printf("  From project: %s\n", dependency.FromProject)
		output.printf("  Manifest path: %s\n", dependency.ManifestPath)
		output.printf("  Ecosystem: %s\n", dependency.Ecosystem)
		output.printf("  Constraint: %s\n", optionalText(dependency.Constraint))
		output.printf("  Scope: %s\n", dependency.Scope)
		output.printf("  Indirect: %t\n", dependency.Indirect)
		output.printf("  Optional: %t\n", dependency.Optional)
		output.printf("  Resolution: %s\n", dependency.Resolution)
		writeTopologyStringList(output, "  Target projects", "    ", dependency.TargetProjects)
		output.printf("  Targets truncated: %t\n", dependency.TargetsTruncated)
	}
}

func writeTopologyMarkers(
	output *textWriter,
	indent string,
	markers []analysis.TopologyMarker,
) {
	output.printf("%sMarkers:\n", indent)
	if len(markers) == 0 {
		output.printf("%s- None\n", indent+"  ")
		return
	}
	for _, marker := range markers {
		output.printf("%s- ID: %s\n", indent+"  ", marker.ID)
		writeTopologyStringList(
			output,
			indent+"    Evidence",
			indent+"      ",
			marker.Evidence,
		)
		output.printf("%s  Evidence truncated: %t\n", indent+"  ", marker.EvidenceTruncated)
	}
}

func writeTopologyComponents(
	output *textWriter,
	indent string,
	components []analysis.TopologyComponent,
) {
	output.printf("%sComponents:\n", indent)
	if len(components) == 0 {
		output.printf("%s- None\n", indent+"  ")
		return
	}
	for _, component := range components {
		output.printf("%s- Manifest path: %s\n", indent+"  ", component.ManifestPath)
		output.printf("%s  Format: %s\n", indent+"  ", component.Format)
		output.printf("%s  Name: %s\n", indent+"  ", optionalText(component.Name))
		output.printf("%s  Version: %s\n", indent+"  ", optionalText(component.Version))
		output.printf("%s  Module: %s\n", indent+"  ", optionalText(component.Module))
		output.printf("%s  Workspace declared: %t\n", indent+"  ", component.WorkspaceDeclared)
		writeTopologyConstraints(output, indent+"    ", component.Constraints)
		output.printf("%s  Dependencies truncated: %t\n", indent+"  ", component.DependenciesTruncated)
		output.printf("%s  Constraints truncated: %t\n", indent+"  ", component.ConstraintsTruncated)
		output.printf("%s  Workspace members truncated: %t\n", indent+"  ", component.WorkspaceMembersTruncated)
		output.printf("%s  Workspace excludes truncated: %t\n", indent+"  ", component.WorkspaceExcludesTruncated)
	}
}

func writeTopologyConstraints(
	output *textWriter,
	indent string,
	constraints []analysis.TopologyConstraint,
) {
	output.printf("%sConstraints:\n", indent)
	if len(constraints) == 0 {
		output.printf("%s- None\n", indent+"  ")
		return
	}
	for _, constraint := range constraints {
		output.printf("%s- Name: %s\n", indent+"  ", constraint.Name)
		output.printf("%s  Value: %s\n", indent+"  ", optionalText(constraint.Value))
		output.printf("%s  Scope: %s\n", indent+"  ", constraint.Scope)
	}
}

func writeTopologyDeclarations(
	output *textWriter,
	declarations []analysis.TopologyWorkspaceDeclaration,
) {
	output.line("  Declarations:")
	if len(declarations) == 0 {
		output.line("    - None")
		return
	}
	for _, declaration := range declarations {
		output.printf("    - Manifest path: %s\n", declaration.ManifestPath)
		output.printf("      Pattern: %s\n", declaration.Pattern)
		output.printf("      Exclude: %t\n", declaration.Exclude)
		output.printf("      Resolution: %s\n", declaration.Resolution)
		writeTopologyStringList(output, "      Project roots", "        ", declaration.ProjectRoots)
		output.printf("      Matches truncated: %t\n", declaration.MatchesTruncated)
	}
}

func writeTopologyStringList(
	output *textWriter,
	label string,
	itemIndent string,
	values []string,
) {
	if len(values) == 0 {
		output.printf("%s: none\n", label)
		return
	}
	output.printf("%s:\n", label)
	for _, value := range values {
		output.printf("%s- %s\n", itemIndent, value)
	}
}

func optionalText(value string) string {
	if value == "" {
		return "none"
	}
	return value
}
