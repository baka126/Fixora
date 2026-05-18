package analyzer

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestResourceQuotaAnalyzerFlagsExhaustedQuota(t *testing.T) {
	client := fake.NewSimpleClientset(&v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "compute", Namespace: "default"},
		Status: v1.ResourceQuotaStatus{
			Hard: v1.ResourceList{v1.ResourcePods: resource.MustParse("10")},
			Used: v1.ResourceList{v1.ResourcePods: resource.MustParse("10")},
		},
	})
	results, err := (&ResourceQuotaAnalyzer{}).Analyze(Context{Client: client, Context: context.Background(), Namespace: "default"})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if len(results) != 1 || results[0].Kind != "ResourceQuota" {
		t.Fatalf("results = %#v, want exhausted ResourceQuota finding", results)
	}
}
