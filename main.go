package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wI2L/jsondiff"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/validate-pods", ServeValidatePods)
	http.HandleFunc("/mutate-pods", ServeMutatePods)

	cert := "/etc/certs/webhook/tls.crt"
	key := "/etc/certs/webhook/tls.key"
	log.Fatalln(http.ListenAndServeTLS(":8443", cert, key, nil))
}

// ServeValidatePods validates a pod to make sure a label exists.
func ServeValidatePods(w http.ResponseWriter, r *http.Request) {
	req, err := extractAdmission(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Read Pod data
	p, err := checkForPod(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if the pod requires mutation
	var resp *admissionv1.AdmissionReview
	if _, exists := p.Labels["teacher"]; !exists {
		resp = response(req.Request.UID, false, http.StatusForbidden, "pod label is invalid")
	} else {
		resp = response(req.Request.UID, true, http.StatusAccepted, "valid pod")
	}

	w.Header().Set("Content-Type", "application/json")
	j, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "couldn't marshal admission response", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "%s", j)
}

// ServeMutatePods mutates the pod to add a label if it's missing.
func ServeMutatePods(w http.ResponseWriter, r *http.Request) {
	req, err := extractAdmission(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Read Pod data
	p, err := checkForPod(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pMod := p.DeepCopy()

	// Mutate the labels if required
	var resp *admissionv1.AdmissionReview
	if pMod.Labels == nil || pMod.Labels["super-teacher"] != "Drewbernetes" {
		log.Println("invalid or no super-teacher label found - correcting that error")
		pMod.Labels["super-teacher"] = "Drewbernetes"
	}

	patch, err := jsondiff.Compare(p, pMod)
	if err != nil {
		http.Error(w, "there was an error comparing the mutated pod with the original", http.StatusBadRequest)
		return
	}

	pBytes, err := json.Marshal(patch)
	if err != nil {
		http.Error(w, "there was an error marshalling the patch", http.StatusBadRequest)
		return
	}

	resp = responsePatch(req.Request.UID, pBytes)

	w.Header().Set("Content-Type", "application/json")
	j, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "couldn't marshal admission response", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "%s", j)
}

// checkForPod validates the object is a pod and can be unmarshalled
func checkForPod(req *admissionv1.AdmissionReview) (*corev1.Pod, error) {
	// Read Pod data
	if req.Request.Kind.Kind != "Pod" {
		return nil, errors.New("only pods are supported")
	}

	var p corev1.Pod
	if err := json.Unmarshal(req.Request.Object.Raw, &p); err != nil {
		return nil, errors.New("couldn't read pod")
	}
	return &p, nil
}

// extractAdmission extracts the admission request.
func extractAdmission(r *http.Request) (*admissionv1.AdmissionReview, error) {
	if r.Header.Get("Content-Type") != "application/json" {
		return nil, fmt.Errorf("incorrect content type %q - Should be %s", r.Header.Get("Content-Type"), "application/json")
	}

	buff := new(bytes.Buffer)
	buff.ReadFrom(r.Body)
	body := buff.Bytes()

	if len(body) == 0 {
		return nil, fmt.Errorf("body is empty")
	}

	var adm admissionv1.AdmissionReview
	if err := json.Unmarshal(body, &adm); err != nil {
		return nil, fmt.Errorf("couldn't read the admission review request: %s", err)
	}
	fmt.Println(adm)

	if adm.Request == nil {
		return nil, fmt.Errorf("admission request is nil")
	}

	return &adm, nil
}

// response generates a response for the AdmissionReview validation.
func response(uid types.UID, allowed bool, code int32,
	message string) *admissionv1.AdmissionReview {
	return &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1",
		},
		Response: &admissionv1.AdmissionResponse{
			UID:     uid,
			Allowed: allowed,
			Result: &metav1.Status{
				Code:    code,
				Message: message,
			},
		},
	}
}

// responsePatch generates a patch for the AdmissionReview mutation.
func responsePatch(uid types.UID, patch []byte) *admissionv1.AdmissionReview {
	pType := admissionv1.PatchTypeJSONPatch
	return &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1",
		},
		Response: &admissionv1.AdmissionResponse{
			UID:       uid,
			Allowed:   true,
			PatchType: &pType,
			Patch:     patch,
		},
	}
}
