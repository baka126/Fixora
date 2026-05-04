package telemetry

import "github.com/prometheus/client_golang/prometheus"

var (
	InvestigationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fixora_investigations_total",
			Help: "Total Fixora investigations by phase and category.",
		},
		[]string{"phase", "category"},
	)
	RemediationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fixora_remediations_total",
			Help: "Total Fixora remediation lifecycle events by status and strategy.",
		},
		[]string{"status", "strategy"},
	)
	ValidationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fixora_validation_total",
			Help: "Total Fixora validation results by validator and result.",
		},
		[]string{"validator", "result"},
	)
	PolicyRejectionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fixora_policy_rejections_total",
			Help: "Total Fixora policy guardrail rejections by reason.",
		},
		[]string{"reason"},
	)
)

func init() {
	prometheus.MustRegister(InvestigationsTotal)
	prometheus.MustRegister(RemediationsTotal)
	prometheus.MustRegister(ValidationTotal)
	prometheus.MustRegister(PolicyRejectionsTotal)
}

func IncInvestigation(phase, category string) {
	InvestigationsTotal.WithLabelValues(label(phase), label(category)).Inc()
}

func IncRemediation(status, strategy string) {
	RemediationsTotal.WithLabelValues(label(status), label(strategy)).Inc()
}

func IncValidation(validator, result string) {
	ValidationTotal.WithLabelValues(label(validator), label(result)).Inc()
}

func IncPolicyRejection(reason string) {
	PolicyRejectionsTotal.WithLabelValues(label(reason)).Inc()
}

func label(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
