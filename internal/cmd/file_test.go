package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fileJSON = `{"id":12,"uuid":"474cc93a-2d6f-47e9-bd3f-a5b5a1941314","name":"scan",` +
	`"size_in_bytes":218476,"extension":"pdf","mime_type":"application/pdf","user_id":4,` +
	`"is_archived":false,"is_referenced":true,"source_type":"web",` +
	`"created_at":"2026-06-09T08:52:10+00:00"}`

func TestFileListTable(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3.0/files" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("archived_state"); got != "all" {
			t.Errorf("archived_state = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		w.Write([]byte("[" + fileJSON + "]"))
	}), "file", "list", "--archived-state", "all")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "scan") || !strings.Contains(out, "application/pdf") || !strings.Contains(out, "218476") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestFileListInvalidArchivedState(t *testing.T) {
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	}), "file", "list", "--archived-state", "nope")
	if err == nil || !strings.Contains(err.Error(), "invalid archived state") {
		t.Fatalf("err = %v", err)
	}
}

func TestFileUploadSendsMultipart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scan.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4 fake"), 0o600); err != nil {
		t.Fatal(err)
	}

	var gotField, gotName, gotContent, gotContentType string
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3.0/files" || r.Method != http.MethodPost {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		gotContentType = r.Header.Get("Content-Type")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		for field, headers := range r.MultipartForm.File {
			gotField = field
			gotName = headers[0].Filename
			f, err := headers[0].Open()
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(f)
			_ = f.Close()
			gotContent = string(b)
		}
		// The upload endpoint answers with an array, not a bare object.
		w.Write([]byte("[" + fileJSON + "]"))
	}), "file", "upload", path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(gotContentType, "multipart/form-data;") {
		t.Fatalf("Content-Type = %q", gotContentType)
	}
	if gotField != "file" {
		t.Fatalf("multipart field = %q, want \"file\"", gotField)
	}
	if gotName != "scan.pdf" {
		t.Fatalf("filename = %q", gotName)
	}
	if gotContent != "%PDF-1.4 fake" {
		t.Fatalf("content = %q", gotContent)
	}
	if !strings.Contains(out, "Uploaded file 12") {
		t.Fatalf("output: %s", out)
	}
}

func TestFileUploadNameOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scan.pdf")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	var gotName string
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		gotName = r.MultipartForm.File["file"][0].Filename
		w.Write([]byte(fileJSON))
	}), "file", "upload", path, "--name", "receipt.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if gotName != "receipt.pdf" {
		t.Fatalf("filename = %q", gotName)
	}
}

func TestFileUploadReadOnlyBlocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scan.pdf")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BEXIO_READ_ONLY", "1")
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected in read-only mode")
	}), "file", "upload", path)
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("err = %v", err)
	}
}

func TestFileDownloadWritesOut(t *testing.T) {
	out := filepath.Join(t.TempDir(), "downloaded.pdf")
	stdout, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3.0/files/12/download" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="scan.pdf"`)
		w.Write([]byte("raw-bytes-not-base64"))
	}), "file", "download", "12", "--out", out)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "raw-bytes-not-base64" {
		t.Fatalf("file content = %q", data)
	}
	if !strings.Contains(stdout, "Wrote "+out) {
		t.Fatalf("output: %s", stdout)
	}
}

func TestFilePreviewToStdout(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3.0/files/12/preview" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("PNGDATA"))
	}), "file", "preview", "12", "--out", "-")
	if err != nil {
		t.Fatal(err)
	}
	if out != "PNGDATA" {
		t.Fatalf("output = %q", out)
	}
}

func TestFileDeleteRequiresForce(t *testing.T) {
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected without --force")
	}), "file", "delete", "12")
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("err = %v", err)
	}

	var method, path string
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.Write([]byte(`{"success":true}`))
	}), "file", "delete", "12", "--force")
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodDelete || path != "/3.0/files/12" {
		t.Fatalf("%s %s", method, path)
	}
	if !strings.Contains(out, "Deleted file 12") {
		t.Fatalf("output: %s", out)
	}
}

func TestFileUpdateUsesPatch(t *testing.T) {
	var method string
	var body map[string]any
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Error(err)
		}
		w.Write([]byte(fileJSON))
	}), "file", "update", "12", "--name", "contract.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPatch {
		t.Fatalf("method = %s", method)
	}
	if len(body) != 1 || body["name"] != "contract.pdf" {
		t.Fatalf("payload = %v", body)
	}
}

func TestFileSearchBuildsCriteria(t *testing.T) {
	var body string
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3.0/files/search" {
			t.Errorf("path = %q", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Write([]byte(`[]`))
	}), "file", "search", "scan", "-w", "mime_type=application/pdf")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `{"field":"mime_type","value":"application/pdf","criteria":"equal"}`) {
		t.Fatalf("missing where clause: %s", body)
	}
	if !strings.Contains(body, `{"field":"name","value":"%scan%","criteria":"like"}`) {
		t.Fatalf("missing term clause: %s", body)
	}
}

func TestFileUsageAcceptsObjectAndArray(t *testing.T) {
	const usage = `{"id":4,"ref_class":"KbInvoice","title":"RE-00001","document_nr":"RE-00001"}`
	for _, payload := range []string{usage, "[" + usage + "]"} {
		out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/3.0/files/12/usage" {
				t.Errorf("path = %q", r.URL.Path)
			}
			w.Write([]byte(payload))
		}), "file", "usage", "12")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "KbInvoice") || !strings.Contains(out, "RE-00001") {
			t.Fatalf("output:\n%s", out)
		}
	}
}

func TestFileViewJSON(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3.0/files/12" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(fileJSON))
	}), "file", "view", "12", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if m["uuid"] != "474cc93a-2d6f-47e9-bd3f-a5b5a1941314" {
		t.Fatalf("got %v", m)
	}
}
