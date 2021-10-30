package server

import (
	"io"
	"net/http"
	"log"

	admv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/json"
)

func updateReview(review *admv1beta1.AdmissionReview) {
	var pod corev1.Pod
	if err := json.Unmarshal(review.Request.Object.Raw, &pod); err != nil {
		log.Printf("[ERROR] Dump req's object failed: %s\n", err)
		return
	}

	// check if annotations match the injection criteria
	annos := pod.ObjectMeta.Annotations
	if annos == nil || annos[INJECT_STATUS] == "true" || annos[INJECT_LABEL] != "true" {
		log.Println("[INFO] Skip sidecar injection!")
		return
	}

	patch := []Patch_t{}

	hasContainer := len(pod.Spec.Containers) != 0
	hasVolume := len(pod.Spec.Volumes) != 0

	// patch sidecar container(s)
	for i := range Sidecarspec.Containers {
		if hasContainer {
			patch = append(patch, Patch_t{Op: "add", Path: CNTPATHAPPEND, Value: Sidecarspec.Containers[i],})
		}else{
			patch = append(patch, Patch_t{Op: "add", Path: CNTPATH, Value: []corev1.Container{Sidecarspec.Containers[i]},})
			hasContainer = false
		}
	}

	// patch sidecar volume(s)
	for i := range Sidecarspec.Volumes {
		if hasVolume {
			patch = append(patch, Patch_t{Op: "add", Path: VOLPATHAPPEND, Value: Sidecarspec.Volumes[i],})
		}else{
			patch = append(patch, Patch_t{Op: "add", Path: VOLPATH, Value: []corev1.Volume{Sidecarspec.Volumes[i]},})
			hasVolume = false
		}
	}

	// patch annotation
	if annos[INJECT_STATUS] == "" {
		patch = append(patch, Patch_t{Op: "add", Path: "/metadata/annotations/-", Value: map[string]string{ INJECT_STATUS: "true", },})
	}else{
		patch = append(patch, Patch_t{Op: "replace", Path: "/metadata/annotations/"+INJECT_STATUS, Value: "true",})
	}

	// serialize patch
	patchbuf, err := json.Marshal(patch)
	if err != nil {
		log.Printf("[ERROR] Serialization patch: %s\n", err)
		return
	}

	// write response
	var patchtype admv1beta1.PatchType = "JSONPatch"
	review.Response = &admv1beta1.AdmissionResponse{ UID: review.Request.UID, Allowed: true, Patch: patchbuf, PatchType: &patchtype, }

	log.Println("mutation handler to implement")
}

func muteHandler(w http.ResponseWriter, r *http.Request) {
	// 1) read request's body into buffer
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		log.Fatalln(err)
	}
	// defer r.Body.Close()

	// 2) check body's content
	if len(buf) == 0 {
		log.Println("[WARN] Empty body!")
		http.Error(w, "Empty body", http.StatusBadRequest)
		return
	}

	ct := r.Header.Get("Content-Type")
	if ct != "application/json" {
		log.Printf("[WARN] Need application/json type, received type: %s\n", ct)
		http.Error(w, "Expect application/json type data", http.StatusBadRequest)
		return
	}

	// 3) decode to AdmissionReview
	review := &admv1beta1.AdmissionReview{}
	decoder := serializer.NewCodecFactory(Scheme).UniversalDeserializer()
	_, _, err = decoder.Decode(buf, nil, review)
	if err != nil {
		log.Printf("[WARN] Decode body to admissionreview failed: %s\n", err)
		http.Error(w, "Json content malformed", http.StatusBadRequest)
		return
	}

	// 4) handle AdmissionReview
	updateReview(review)

	// 5) Prepare response
	rspbuf, err := json.Marshal(review)
	if err != nil {
		log.Printf("[ERROR] Serialize admissionreview failed: %s\n", err)
		http.Error(w, "Prepare response failed", http.StatusInternalServerError)
		return
	}

	if _, err := w.Write(rspbuf); err != nil {
		log.Printf("[ERROR] Serialize admissionreview failed: %s\n", err)
		http.Error(w, "Prepare response failed", http.StatusInternalServerError)
	}
}
