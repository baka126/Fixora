import re

with open('pkg/controller/controller.go', 'r') as f:
    content = f.read()

# Define the mapping
mapping_code = """
var failureToFieldsMap = map[string][]string{
	"OOMKilled":                 {"resources"},
	"CPUThrottling":            {"resources"},
	"ImagePullBackOff":          {"image", "imagePullPolicy"},
	"ErrImagePull":              {"image", "imagePullPolicy"},
	"CreateContainerConfigError": {"env", "envFrom", "volumeMounts", "command", "args"},
	"CreateContainerError":       {"env", "envFrom", "volumeMounts", "command", "args"},
	"Pending (Unschedulable)":    {"nodeSelector", "tolerations", "affinity", "resources"},
	"NodeNotReady":              {"nodeSelector", "tolerations", "affinity"},
}
"""

# Insert mapping after imports or before Controller struct
if "var failureToFieldsMap" not in content:
    content = content.replace("type podWorkItem struct {", mapping_code + "\\ntype podWorkItem struct {")

# Update the loop in handleRemediation
old_snippet_logic = """			// TARGETED REMEDIATION: If OOM/CPU issues, extract only the resources block
			isResourceIssue := strings.Contains(evidence.ClusterContext, "OOMKilled") || 
							  strings.Contains(evidence.ClusterContext, "CPUThrottling") ||
							  strings.Contains(evidence.ClusterContext, "DeadlineExceeded")
			
			if isResourceIssue && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
				// Try to extract container name from pod (usually first container or pod name)
				containerName := pod.Name
				if len(pod.Spec.Containers) > 0 {
					containerName = pod.Spec.Containers[0].Name
				}
				
				snippet, err := ai.ExtractResources(contentStr, containerName)
				if err == nil {
					slog.Info("Extracted targeted resource context", "file", path, "container", containerName)
					contentStr = snippet
				}
			} else if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
				contentStr = ai.CompressYAML(contentStr)
			}"""

new_snippet_logic = """			// TARGETED REMEDIATION: Surgically extract relevant fields based on failure reason
			var targetFields []string
			for reason, fields := range failureToFieldsMap {
				if strings.Contains(evidence.ClusterContext, reason) {
					targetFields = fields
					break
				}
			}

			if len(targetFields) > 0 && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
				containerName := pod.Name
				if len(pod.Spec.Containers) > 0 {
					containerName = pod.Spec.Containers[0].Name
				}
				snippet, err := ai.ExtractSnippet(contentStr, containerName, targetFields)
				if err == nil {
					slog.Info("Extracted targeted failure context", "file", path, "fields", targetFields)
					contentStr = snippet
				}
			} else if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
				contentStr = ai.CompressYAML(contentStr)
			}"""

content = content.replace(old_snippet_logic, new_snippet_logic)

# Update the merge logic in handleRemediation
old_merge_logic = """		// TARGETED MERGE: If AI returned only a resource block, merge it back to original file
		for repoURL, info := range deps {
			u, _ := giturls.Parse(repoURL)
			if strings.Contains(u.Path, p.RepoName) {
				// Get original full content for this file
				origBytes, err := provider.GetFileContent(ctx, p.RepoOwner, p.RepoName, p.FilePath, "main")
				if err == nil {
					// Check if this looks like a resources snippet
					if strings.Contains(content, "limits:") || strings.Contains(content, "requests:") {
						containerName := pod.Name
						if len(pod.Spec.Containers) > 0 {
							containerName = pod.Spec.Containers[0].Name
						}
						merged, err := ai.TargetedUpdate(string(origBytes), containerName, content)
						if err == nil {
							slog.Info("Surgically applied targeted resource update", "file", p.FilePath)
							content = merged
						}
					}
				}
				break
			}
		}"""

new_merge_logic = """		// TARGETED MERGE: Surgically merge AI patches back into the full manifest
		for repoURL := range deps {
			u, _ := giturls.Parse(repoURL)
			if strings.Contains(u.Path, p.RepoName) {
				origBytes, err := provider.GetFileContent(ctx, p.RepoOwner, p.RepoName, p.FilePath, "main")
				if err == nil {
					containerName := pod.Name
					if len(pod.Spec.Containers) > 0 {
						containerName = pod.Spec.Containers[0].Name
					}
					merged, err := ai.SurgicalUpdate(string(origBytes), containerName, content)
					if err == nil {
						slog.Info("Surgically applied targeted GitOps update", "file", p.FilePath)
						content = merged
					}
				}
				break
			}
		}"""

content = content.replace(old_merge_logic, new_merge_logic)

with open('pkg/controller/controller.go', 'w') as f:
    f.write(content)
