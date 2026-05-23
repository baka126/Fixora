package controller

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type dependencyEnvSuggestion struct {
	ServiceName     string
	ServiceHost     string
	ServicePort     int32
	ServicePortName string
	SecretName      string
	SecretKeys      []string
	EnvPrefix       string
	MissingEnv      []string
	CurrentEnv      []string
	Reason          string
}

func (s dependencyEnvSuggestion) Summary() string {
	if s.ServiceName == "" || s.ServicePort <= 0 {
		return ""
	}
	env := []string{
		fmt.Sprintf("%s_HOST=%s", s.EnvPrefix, s.ServiceHost),
		fmt.Sprintf("%s_PORT=%d", s.EnvPrefix, s.ServicePort),
	}
	if s.SecretName != "" {
		env = append(env, fmt.Sprintf("optional credentials from Secret/%s keys=%s", s.SecretName, strings.Join(s.SecretKeys, ",")))
	}
	return fmt.Sprintf("Dependency env candidate: Service %s exposes port %d; recommended env %s. Missing or mismatched env: %s. Reason: %s",
		s.ServiceName,
		s.ServicePort,
		strings.Join(env, ", "),
		strings.Join(s.MissingEnv, ", "),
		s.Reason,
	)
}

func (s dependencyEnvSuggestion) Evidence() []string {
	if s.ServiceName == "" {
		return nil
	}
	out := []string{
		fmt.Sprintf("Discovered dependency Service/%s in the same namespace.", s.ServiceName),
		fmt.Sprintf("Recommended %s_HOST=%s and %s_PORT=%d for the failing container.", s.EnvPrefix, s.ServiceHost, s.EnvPrefix, s.ServicePort),
	}
	if s.SecretName != "" {
		out = append(out, fmt.Sprintf("Discovered credential candidate Secret/%s with keys [%s]; values were not read into evidence.", s.SecretName, strings.Join(s.SecretKeys, ", ")))
	}
	if len(s.MissingEnv) > 0 {
		out = append(out, "Missing or mismatched env vars: "+strings.Join(s.MissingEnv, ", "))
	}
	return out
}

func (c *Controller) dependencyEnvSuggestion(ctx context.Context, pod *v1.Pod, diagnosis Diagnosis, collected CollectedEvidence) (dependencyEnvSuggestion, bool) {
	if c == nil || c.clientset == nil || pod == nil {
		return dependencyEnvSuggestion{}, false
	}
	haystack := strings.ToLower(strings.Join([]string{
		diagnosis.Symptom,
		diagnosis.LikelyCause,
		strings.Join(diagnosis.Evidence, "\n"),
		collected.EventTimeline,
		collected.Logs,
		collected.StackTrace,
	}, "\n"))
	service, serviceScore, ok := c.bestDependencyService(ctx, pod.Namespace, haystack)
	if !ok || serviceScore < 35 {
		return dependencyEnvSuggestion{}, false
	}
	if !hasDependencyFailureSignal(haystack) && serviceScore < 70 {
		return dependencyEnvSuggestion{}, false
	}
	port := bestDependencyServicePort(service, haystack)
	if port.Port <= 0 {
		return dependencyEnvSuggestion{}, false
	}

	envPrefix := dependencyEnvPrefix(service.Name, port.Port)
	hostVar := envPrefix + "_HOST"
	portVar := envPrefix + "_PORT"
	serviceHost := service.Name
	container := targetContainerSpec(pod, failingContainerNameForPod(pod))
	currentEnv := containerEnvSummary(container, hostVar, portVar)
	missing := missingOrMismatchedDependencyEnv(container, hostVar, serviceHost, portVar, strconv.Itoa(int(port.Port)))
	if len(missing) == 0 {
		return dependencyEnvSuggestion{}, false
	}

	secretName, secretKeys := c.bestCredentialSecret(ctx, pod.Namespace, service.Name, haystack)
	suggestion := dependencyEnvSuggestion{
		ServiceName:     service.Name,
		ServiceHost:     serviceHost,
		ServicePort:     port.Port,
		ServicePortName: port.Name,
		SecretName:      secretName,
		SecretKeys:      secretKeys,
		EnvPrefix:       envPrefix,
		MissingEnv:      missing,
		CurrentEnv:      currentEnv,
		Reason:          dependencyFailureReason(haystack),
	}
	return suggestion, true
}

func applyDependencyEnvSuggestion(diagnosis *Diagnosis, corr *ResourceCorrelation, suggestion dependencyEnvSuggestion) {
	if diagnosis == nil || suggestion.ServiceName == "" {
		return
	}
	diagnosis.Category = CategoryConfig
	diagnosis.PatchStrategy = PatchEnvOrVolumeRef
	if diagnosis.Confidence < 82 {
		diagnosis.Confidence = 82
	}
	diagnosis.Symptom = firstNonEmpty(diagnosis.Symptom, "Application cannot connect to a required dependency")
	diagnosis.LikelyCause = fmt.Sprintf("The workload appears to be missing or misconfigured dependency environment variables for Service/%s. Add or update %s_HOST and %s_PORT on the target container; reference existing credentials from Secret/%s when required.",
		suggestion.ServiceName,
		suggestion.EnvPrefix,
		suggestion.EnvPrefix,
		firstNonEmpty(suggestion.SecretName, "<existing-credential-secret>"),
	)
	diagnosis.Evidence = append(diagnosis.Evidence, suggestion.Evidence()...)
	diagnosis.Related = append(diagnosis.Related, "Service/"+suggestion.ServiceName)
	if suggestion.SecretName != "" {
		diagnosis.Related = append(diagnosis.Related, "Secret/"+suggestion.SecretName)
	}
	diagnosis.Related = uniqueSorted(diagnosis.Related)
	if corr != nil {
		corr.relate("Service", suggestion.ServiceName)
		corr.score("Service", suggestion.ServiceName, 80, "dependency service candidate for failing connection")
		if suggestion.SecretName != "" {
			corr.relate("Secret", suggestion.SecretName)
			corr.score("Secret", suggestion.SecretName, 55, "credential candidate for dependency connection")
		}
		corr.add(suggestion.Summary())
	}
}

func (c *Controller) bestDependencyService(ctx context.Context, namespace, haystack string) (v1.Service, int, bool) {
	services, err := c.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return v1.Service{}, 0, false
	}
	var best v1.Service
	var bestScore int
	for _, svc := range services.Items {
		score := dependencyServiceScore(svc, haystack)
		if score > bestScore || (score == bestScore && svc.Name < best.Name) {
			best = svc
			bestScore = score
		}
	}
	return best, bestScore, bestScore > 0
}

func dependencyServiceScore(svc v1.Service, haystack string) int {
	score := 0
	name := strings.ToLower(svc.Name)
	if strings.Contains(haystack, name) {
		score += 45
	}
	if containsAny(name, "db", "database", "postgres", "postgresql", "mysql", "mariadb", "mongo", "mongodb", "redis", "sql") {
		score += 35
	}
	for key, value := range svc.Labels {
		label := strings.ToLower(key + "=" + value)
		if containsAny(label, "db", "database", "postgres", "mysql", "redis", "mongo") {
			score += 15
			break
		}
	}
	for _, port := range svc.Spec.Ports {
		if isKnownDependencyPort(port.Port) {
			score += 20
		}
		if port.Name != "" && containsAny(strings.ToLower(port.Name), "db", "postgres", "mysql", "redis", "mongo") {
			score += 15
		}
	}
	return score
}

func bestDependencyServicePort(svc v1.Service, haystack string) v1.ServicePort {
	var fallback v1.ServicePort
	for _, port := range svc.Spec.Ports {
		if fallback.Port == 0 {
			fallback = port
		}
		name := strings.ToLower(port.Name)
		if name != "" && strings.Contains(haystack, name) {
			return port
		}
		if isKnownDependencyPort(port.Port) {
			return port
		}
	}
	return fallback
}

func (c *Controller) bestCredentialSecret(ctx context.Context, namespace, serviceName, haystack string) (string, []string) {
	secrets, err := c.clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", nil
	}
	type candidate struct {
		name  string
		score int
		keys  []string
	}
	var best candidate
	for _, secret := range secrets.Items {
		score, keys := credentialSecretScore(secret, serviceName, haystack)
		if score > best.score || (score == best.score && secret.Name < best.name) {
			best = candidate{name: secret.Name, score: score, keys: keys}
		}
	}
	if best.score < 25 {
		return "", nil
	}
	sort.Strings(best.keys)
	return best.name, best.keys
}

func credentialSecretScore(secret v1.Secret, serviceName, haystack string) (int, []string) {
	name := strings.ToLower(secret.Name)
	score := 0
	if strings.Contains(haystack, name) {
		score += 35
	}
	if serviceName != "" && strings.Contains(name, strings.ToLower(serviceName)) {
		score += 20
	}
	if containsAny(name, "cred", "credential", "secret", "db", "database", "postgres", "postgresql", "mysql", "mariadb", "mongo", "redis", "sql") {
		score += 25
	}
	keys := make([]string, 0, len(secret.Data))
	for key := range secret.Data {
		lower := strings.ToLower(key)
		if isCredentialKey(lower) {
			score += 10
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		for key := range secret.Data {
			keys = append(keys, key)
			if len(keys) == 3 {
				break
			}
		}
	}
	return score, keys
}

func targetContainerSpec(pod *v1.Pod, failingContainerName string) *v1.Container {
	if pod == nil {
		return nil
	}
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == failingContainerName {
			return &pod.Spec.Containers[i]
		}
	}
	if len(pod.Spec.Containers) > 0 {
		return &pod.Spec.Containers[0]
	}
	return nil
}

func missingOrMismatchedDependencyEnv(container *v1.Container, hostVar, hostValue, portVar, portValue string) []string {
	env := containerEnvMap(container)
	var missing []string
	if !envValueMatchesDependency(env[hostVar], hostValue) {
		missing = append(missing, hostVar)
	}
	if !envValueMatchesDependency(env[portVar], portValue) {
		missing = append(missing, portVar)
	}
	return missing
}

func containerEnvMap(container *v1.Container) map[string]string {
	out := map[string]string{}
	if container == nil {
		return out
	}
	for _, env := range container.Env {
		if env.ValueFrom != nil {
			out[env.Name] = "<valueFrom>"
			continue
		}
		out[env.Name] = strings.TrimSpace(env.Value)
	}
	return out
}

func containerEnvSummary(container *v1.Container, names ...string) []string {
	env := containerEnvMap(container)
	var out []string
	for _, name := range names {
		if value, ok := env[name]; ok {
			if value == "" {
				value = "<empty>"
			}
			out = append(out, name+"="+value)
		}
	}
	return out
}

func envValueMatchesDependency(current, expected string) bool {
	current = strings.TrimSpace(current)
	expected = strings.TrimSpace(expected)
	if current == "" || expected == "" {
		return false
	}
	if current == "<valueFrom>" {
		return true
	}
	if current == expected {
		return true
	}
	return strings.HasPrefix(current, expected+".") ||
		strings.HasPrefix(current, "http://"+expected+":") ||
		strings.HasPrefix(current, "https://"+expected+":")
}

func hasDependencyFailureSignal(haystack string) bool {
	if haystack == "" {
		return false
	}
	connectivity := containsAny(haystack, "connection refused", "connect: refused", "no route to host", "connection timed out", "i/o timeout", "dial tcp", "getaddrinfo", "temporary failure in name resolution")
	auth := containsAny(haystack, "access_denied", "access denied", "authentication failed", "password authentication failed", "permission denied", "login failed", "invalid password")
	database := containsAny(haystack, "database", "db_", "postgres", "postgresql", "mysql", "mariadb", "mongo", "mongodb", "redis", "sqlstate", "jdbc", "dsn")
	return (connectivity || auth) && database
}

func dependencyFailureReason(haystack string) string {
	switch {
	case containsAny(haystack, "access_denied", "access denied", "authentication failed", "password authentication failed", "login failed", "invalid password"):
		return "database authentication failure"
	case containsAny(haystack, "connection refused", "connect: refused", "dial tcp"):
		return "database connection refused"
	case containsAny(haystack, "no route to host", "connection timed out", "i/o timeout"):
		return "database network connection failure"
	default:
		return "database dependency connection failure"
	}
}

func dependencyEnvPrefix(serviceName string, port int32) string {
	name := strings.ToLower(serviceName)
	switch {
	case strings.Contains(name, "redis") || port == 6379:
		return "REDIS"
	case strings.Contains(name, "mongo") || port == 27017:
		return "MONGO"
	case strings.Contains(name, "postgres") || strings.Contains(name, "psql") || port == 5432:
		return "POSTGRES"
	case strings.Contains(name, "mysql") || port == 3306:
		return "MYSQL"
	case strings.Contains(name, "elastic") || port == 9200:
		return "ELASTICSEARCH"
	default:
		return "DB"
	}
}

func isKnownDependencyPort(port int32) bool {
	switch port {
	case 5432, 3306, 6379, 27017, 9200, 9300, 5672, 15672, 9092:
		return true
	default:
		return false
	}
}

func isCredentialKey(key string) bool {
	return containsAny(key, "user", "username", "password", "passwd", "token", "database_url", "db_url", "dsn", "uri", "host", "port", "api_key", "apikey", "secret")
}
