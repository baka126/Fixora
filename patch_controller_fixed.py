import re

with open('pkg/controller/controller.go', 'r') as f:
    content = f.read()

# Fix the TargetedUpdate call
content = content.replace('merged, err := ai.TargetedUpdate(string(origBytes), containerName, content)', 'merged, err := ai.SurgicalUpdate(string(origBytes), containerName, content)')

# Clean up any leftover escape sequences
content = content.replace('\\ntype podWorkItem', 'type podWorkItem')

with open('pkg/controller/controller.go', 'w') as f:
    f.write(content)
