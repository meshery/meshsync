package utils

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// TestGetClusterIDConcurrent runs GetClusterID for several clients at the same
// time, as Meshery Server does when it starts MeshSync for more than one
// Kubernetes context. Run with -race: an unguarded cache fails here.
func TestGetClusterIDConcurrent(t *testing.T) {
	const uid = "3f6c1a52-kube-system"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/namespaces/kube-system" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"kube-system","uid":"` + uid + `"}}`))
	}))
	defer srv.Close()

	type row struct {
		name   string
		client *kubernetes.Clientset
		want   string
	}
	tests := []row{{"nil client has no cluster ID", nil, ""}}
	for _, name := range []string{"client A", "client B", "client C", "client D"} {
		c, err := kubernetes.NewForConfig(&rest.Config{Host: srv.URL})
		if err != nil {
			t.Fatalf("NewForConfig: %v", err)
		}
		tests = append(tests, row{name, c, uid})
	}

	var wg sync.WaitGroup
	got := make([][]string, len(tests))
	for i, tc := range tests {
		got[i] = make([]string, 8)
		for j := range got[i] {
			wg.Add(1)
			go func() {
				defer wg.Done()
				got[i][j] = GetClusterID(tc.client)
			}()
		}
	}
	wg.Wait()

	for i, tc := range tests {
		for j, id := range got[i] {
			if id != tc.want {
				t.Errorf("%s, call %d: GetClusterID = %q, want %q", tc.name, j, id, tc.want)
			}
		}
	}
}
