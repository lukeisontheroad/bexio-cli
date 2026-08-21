package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// File is an uploaded file (API resource "files", 3.0 API). Raw preserves
// the full API object.
type File struct {
	ID            int    `json:"id"`
	UUID          string `json:"uuid"`
	Name          string `json:"name"`
	SizeInBytes   int64  `json:"size_in_bytes"`
	Extension     string `json:"extension"`
	MimeType      string `json:"mime_type"`
	UploaderEmail string `json:"uploader_email"`
	UserID        int    `json:"user_id"`
	IsArchived    bool   `json:"is_archived"`
	SourceID      int    `json:"source_id"`
	SourceType    string `json:"source_type"`
	IsReferenced  bool   `json:"is_referenced"`
	CreatedAt     string `json:"created_at"`

	Raw json.RawMessage `json:"-"`
}

func (f *File) UnmarshalJSON(data []byte) error {
	type file File
	var v file
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*f = File(v)
	f.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// FileUsageRef is one document a file is attached to
// (GET /3.0/files/{id}/usage).
type FileUsageRef struct {
	ID         int    `json:"id"`
	RefClass   string `json:"ref_class"`
	Title      string `json:"title"`
	DocumentNr string `json:"document_nr"`

	Raw json.RawMessage `json:"-"`
}

func (u *FileUsageRef) UnmarshalJSON(data []byte) error {
	type usage FileUsageRef
	var v usage
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*u = FileUsageRef(v)
	u.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// FileArchivedStates are the accepted values of the archived_state filter of
// the file list/search endpoints (files use this instead of the 2.0
// show_archived flag).
var FileArchivedStates = []string{"all", "archived", "not_archived"}

// FileListOptions are the query parameters of GET /3.0/files and
// POST /3.0/files/search.
type FileListOptions struct {
	ListOptions
	// ArchivedState is one of FileArchivedStates; empty leaves the server
	// default (not archived).
	ArchivedState string
}

func (o FileListOptions) fileValues() (url.Values, error) {
	q := o.values()
	if o.ArchivedState != "" {
		if !fileValidArchivedState(o.ArchivedState) {
			return nil, fmt.Errorf("invalid archived state %q (want %s)", o.ArchivedState, strings.Join(FileArchivedStates, ", "))
		}
		q.Set("archived_state", o.ArchivedState)
	}
	return q, nil
}

func fileValidArchivedState(s string) bool {
	for _, v := range FileArchivedStates {
		if v == s {
			return true
		}
	}
	return false
}

// ListFiles fetches GET /3.0/files.
func (c *Client) ListFiles(ctx context.Context, opts FileListOptions) ([]File, error) {
	q, err := opts.fileValues()
	if err != nil {
		return nil, err
	}
	var out []File
	return out, c.Get(ctx, "/3.0/files", q, &out)
}

// SearchFiles posts search criteria to /3.0/files/search.
func (c *Client) SearchFiles(ctx context.Context, criteria []SearchCriterion, opts FileListOptions) ([]File, error) {
	q, err := opts.fileValues()
	if err != nil {
		return nil, err
	}
	var out []File
	return out, c.Do(ctx, http.MethodPost, "/3.0/files/search", q, criteria, &out)
}

// GetFile fetches a single file's metadata.
func (c *Client) GetFile(ctx context.Context, id int) (*File, error) {
	var out File
	return &out, c.Get(ctx, fmt.Sprintf("/3.0/files/%d", id), nil, &out)
}

// UpdateFile edits a file's metadata. Unlike the 2.0 resources this is a
// PATCH; only the provided fields change (name, is_archived, source_type).
func (c *Client) UpdateFile(ctx context.Context, id int, fields map[string]any) (*File, error) {
	var out File
	return &out, c.Do(ctx, http.MethodPatch, fmt.Sprintf("/3.0/files/%d", id), nil, fields, &out)
}

// DeleteFile deletes a file. The API has no restore endpoint for files.
func (c *Client) DeleteFile(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("/3.0/files/%d", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete file %d: API reported failure", id)
	}
	return nil
}

// FileUsage lists the documents a file is attached to. The endpoint returns
// a single object or an array depending on the file, both are normalized to
// a slice here.
func (c *Client) FileUsage(ctx context.Context, id int) ([]FileUsageRef, error) {
	var raw json.RawMessage
	if err := c.Get(ctx, fmt.Sprintf("/3.0/files/%d/usage", id), nil, &raw); err != nil {
		return nil, err
	}
	return fileDecodeList[FileUsageRef](raw)
}

// FileUploadFieldName is the multipart field the upload endpoint expects.
// The spec is ambiguous: the OpenAPI 3 request body calls it "file" while a
// leftover Swagger 2 formData parameter calls it "upFile".
const FileUploadFieldName = "file"

// UploadFile uploads a local file via multipart/form-data to POST /3.0/files.
// name overrides the transmitted filename (defaults to the base name of
// path); field is the multipart field name (defaults to FileUploadFieldName).
func (c *Client) UploadFile(ctx context.Context, path, name, field string) (*File, error) {
	f, err := os.Open(path) //nolint:gosec // the path is a CLI argument
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only handle
	if name == "" {
		name = filepath.Base(path)
	}
	if field == "" {
		field = FileUploadFieldName
	}

	var raw json.RawMessage
	if err := c.DoMultipart(ctx, "/3.0/files", nil, field, name, f, &raw); err != nil {
		return nil, err
	}
	// The upload endpoint answers with an array of file objects, single
	// fetches with a bare object — accept both.
	files, err := fileDecodeList[File](raw)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("upload %s: API returned no file object", name)
	}
	return &files[0], nil
}

// fileDecodeList decodes a response that is either a single object or an
// array of objects into a slice.
func fileDecodeList[T any](raw json.RawMessage) ([]T, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var list []T
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		return list, nil
	}
	var one T
	if err := json.Unmarshal(trimmed, &one); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return []T{one}, nil
}

// FileContent is the raw payload of the download and preview endpoints.
// Unlike the document PDF endpoints these return the bytes directly, not a
// base64 JSON envelope.
type FileContent struct {
	Name        string
	ContentType string
	Data        []byte
}

// DownloadFile fetches the file's content.
func (c *Client) DownloadFile(ctx context.Context, id int) (*FileContent, error) {
	return c.fileRawGet(ctx, fmt.Sprintf("/3.0/files/%d/download", id))
}

// PreviewFile fetches the file's preview image.
func (c *Client) PreviewFile(ctx context.Context, id int) (*FileContent, error) {
	return c.fileRawGet(ctx, fmt.Sprintf("/3.0/files/%d/preview", id))
}

// fileRawGet performs a GET that returns arbitrary bytes instead of JSON.
// Auth, verbose logging, and error mapping mirror Client.Do.
func (c *Client) fileRawGet(ctx context.Context, path string) (*FileContent, error) {
	u := *c.baseURL
	u.Path = strings.TrimRight(u.Path, "/") + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if err := c.fileAuthorize(ctx, req); err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "> %s %s\n", http.MethodGet, u.String())
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if c.Verbose {
		fmt.Fprintf(os.Stderr, "< HTTP %d (%d bytes)\n", resp.StatusCode, len(data))
	}
	if resp.StatusCode >= 400 {
		apiErr := &Error{StatusCode: resp.StatusCode}
		_ = json.Unmarshal(data, apiErr) // best effort; error text may not be JSON
		return nil, apiErr
	}

	return &FileContent{
		Name:        fileNameFromDisposition(resp.Header.Get("Content-Disposition")),
		ContentType: resp.Header.Get("Content-Type"),
		Data:        data,
	}, nil
}

// fileNameFromDisposition extracts the filename of a Content-Disposition
// header, returning "" when there is none.
func fileNameFromDisposition(header string) string {
	if header == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(header)
	if err != nil {
		return ""
	}
	return filepath.Base(params["filename"])
}

// DoMultipart performs a multipart/form-data POST with a single file part.
// Client.Do always JSON-encodes its body, so uploads use this instead; the
// read-only guard, authentication, verbose logging, and error mapping are
// identical.
func (c *Client) DoMultipart(ctx context.Context, path string, query url.Values, field, filename string, content io.Reader, out any) error {
	if c.ReadOnly {
		return fmt.Errorf("read-only instance: refusing %s %s (log in again without --read-only to allow writes)", http.MethodPost, path)
	}
	u := *c.baseURL
	u.Path = strings.TrimRight(u.Path, "/") + path
	if query != nil {
		u.RawQuery = query.Encode()
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile(field, filename)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, content); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body.Bytes()))
	if err != nil {
		return err
	}
	if err := c.fileAuthorize(ctx, req); err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", mw.FormDataContentType())

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "> %s %s (multipart, %d bytes)\n", http.MethodPost, u.String(), body.Len())
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if c.Verbose {
		fmt.Fprintf(os.Stderr, "< HTTP %d (%d bytes)\n", resp.StatusCode, len(data))
	}
	if resp.StatusCode >= 400 {
		apiErr := &Error{StatusCode: resp.StatusCode}
		_ = json.Unmarshal(data, apiErr) // best effort; error text may not be JSON
		return apiErr
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// fileAuthorize sets the standard request headers of Client.Do.
func (c *Client) fileAuthorize(ctx context.Context, req *http.Request) error {
	token, err := c.source.Token(ctx)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "bexio-cli")
	return nil
}
