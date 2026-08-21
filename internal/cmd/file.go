package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lukeisontheroad/bexio-cli/internal/api"
	"github.com/lukeisontheroad/bexio-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() { registerModule(newFileCmd) }

// newFileCmd manages uploaded files (API resource "files", 3.0 API).
func newFileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "file",
		Aliases: []string{"files"},
		Short:   "List, upload, download, and manage files",
		Long: `Manage the files of the bexio file manager (/3.0/files).

Files are referenced by their numeric id (the API also returns a uuid, but
every endpoint takes the id). Uploads send the local file as
multipart/form-data; download and preview return the raw bytes, not a
base64 envelope like the document PDF endpoints.`,
	}
	cmd.AddCommand(
		newFileListCmd(),
		newFileViewCmd(),
		newFileSearchCmd(),
		newFileUploadCmd(),
		newFileUpdateCmd(),
		newFileDeleteCmd(),
		newFileDownloadCmd(),
		newFilePreviewCmd(),
		newFileUsageCmd(),
	)
	return cmd
}

var fileDetailOrder = []string{
	"id", "uuid", "name", "extension", "mime_type", "size_in_bytes",
	"is_archived", "is_referenced", "user_id", "uploader_email",
	"source_type", "source_id", "created_at",
}

const fileOrderFields = "id, created_at, source_id, uuid, name, size_in_bytes"

func renderFiles(cmd *cobra.Command, files []api.File) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(files))
		for i, f := range files {
			raws[i] = f.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(files))
	for i, f := range files {
		rows[i] = []string{
			strconv.Itoa(f.ID),
			output.Truncate(f.Name, 40),
			f.Extension,
			strconv.FormatInt(f.SizeInBytes, 10),
			f.MimeType,
			strconv.FormatBool(f.IsArchived),
			shortDate(f.CreatedAt),
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "name", "ext", "size", "mime_type", "archived", "created"}, rows)
	return nil
}

// fileListFlags adds the shared list/search parameters of /3.0/files.
func fileListFlags(cmd *cobra.Command, opts *api.FileListOptions) {
	listFlags(cmd, &opts.ListOptions, fileOrderFields)
	cmd.Flags().StringVar(&opts.ArchivedState, "archived-state", "",
		"archived filter: all, archived, not_archived (default: server default)")
}

// fileFieldFlags mirrors the writable fields of PATCH /3.0/files/{id}.
type fileFieldFlags struct {
	name       string
	isArchived bool
	sourceType string
}

func (f *fileFieldFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.name, "name", "", "file name")
	fl.BoolVar(&f.isArchived, "is-archived", false, "archive the file (--is-archived=false un-archives)")
	fl.StringVar(&f.sourceType, "source-type", "", "upload source: web, email, mobile")
}

func (f *fileFieldFlags) payload(cmd *cobra.Command) (map[string]any, error) {
	fields := map[string]any{}
	setIfChanged(cmd, fields, "name", "name", f.name)
	setIfChanged(cmd, fields, "is-archived", "is_archived", f.isArchived)
	setIfChanged(cmd, fields, "source-type", "source_type", f.sourceType)
	if cmd.Flags().Changed("source-type") && !fileValidSourceType(f.sourceType) {
		return nil, fmt.Errorf("invalid --source-type %q (want web, email, or mobile)", f.sourceType)
	}
	return fields, nil
}

func fileValidSourceType(s string) bool {
	return s == "web" || s == "email" || s == "mobile"
}

func newFileListCmd() *cobra.Command {
	var opts api.FileListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List files",
		Example: `  bexio file list
  bexio file list --archived-state all --order-by created_at_desc --limit 20 -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			files, err := client.ListFiles(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderFiles(cmd, files)
		},
	}
	fileListFlags(cmd, &opts)
	return cmd
}

func newFileViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a file's metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("file", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			f, err := client.GetFile(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, f.Raw, fileDetailOrder)
		},
	}
}

func newFileSearchCmd() *cobra.Command {
	var opts api.FileListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search [term]",
		Short: "Search files",
		Long: `Search files. A bare term matches the name partially. --where clauses
use the raw API field names and add AND conditions (see
"bexio contact search --help" for the operator syntax).

Searchable fields: uuid, name, extension, mime_type, size_in_bytes,
is_archived, is_referenced, source_type, user_id, created_at.`,
		Example: `  bexio file search invoice
  bexio file search --where mime_type=application/pdf
  bexio file search --where "created_at>2026-01-01" -o json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			criteria, err := parseWhere(where)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				criteria = append(criteria, api.SearchCriterion{
					Field: "name", Value: "%" + args[0] + "%", Criteria: "like",
				})
			}
			if len(criteria) == 0 {
				return fmt.Errorf("nothing to search: give a term or at least one --where clause")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			files, err := client.SearchFiles(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderFiles(cmd, files)
		},
	}
	fileListFlags(cmd, &opts)
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause (repeatable, ANDed); see long help")
	return cmd
}

func newFileUploadCmd() *cobra.Command {
	var name, field string
	cmd := &cobra.Command{
		Use:   "upload <path>",
		Short: "Upload a local file",
		Long: `Upload a local file via multipart/form-data (POST /3.0/files).

The multipart field is named "file"; the bexio spec also documents the
legacy name "upFile" for the same parameter, use --form-field if your
account expects it.`,
		Example: `  bexio file upload ./scan.pdf
  bexio file upload ./scan.pdf --name receipt-2026-01.pdf -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			f, err := client.UploadFile(cmd.Context(), args[0], name, field)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), f.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Uploaded file %d (%s, %d bytes)\n", f.ID, f.Name, f.SizeInBytes)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "file name to store (default: base name of <path>)")
	cmd.Flags().StringVar(&field, "form-field", api.FileUploadFieldName, `multipart field name ("file" or "upFile")`)
	return cmd
}

func newFileUpdateCmd() *cobra.Command {
	var fields fileFieldFlags
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a file's metadata",
		Long:  "Update a file (PATCH /3.0/files/{id}). Only the flags you pass are changed.",
		Example: `  bexio file update 12 --name contract.pdf
  bexio file update 12 --is-archived
  bexio file update 12 --is-archived=false`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("file", args[0])
			if err != nil {
				return err
			}
			payload, err := fields.payload(cmd)
			if err != nil {
				return err
			}
			if len(payload) == 0 {
				return fmt.Errorf("nothing to update: pass at least one field flag")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			f, err := client.UpdateFile(cmd.Context(), id, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), f.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated file %d (%s)\n", f.ID, f.Name)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newFileDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a file",
		Long: `Delete a file. The API offers no restore endpoint for files, so this
cannot be undone from the CLI and --force is required. To keep the file but
hide it, archive it instead: "bexio file update <id> --is-archived".`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("file", args[0])
			if err != nil {
				return err
			}
			if !force {
				return fmt.Errorf("deleting a file cannot be undone (no restore endpoint): re-run with --force")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteFile(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted file %d\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "confirm the deletion")
	return cmd
}

func newFileDownloadCmd() *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "download <id>",
		Short: "Download a file's content",
		Long: `Download a file (GET /3.0/files/{id}/download). The endpoint returns the
raw bytes; the output file name comes from the Content-Disposition header,
falling back to the file's name in the API and finally to "file-<id>".`,
		Example: `  bexio file download 12
  bexio file download 12 --out ./scan.pdf`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fileWriteContent(cmd, args[0], out, false)
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "output file (default: name from the API, - for stdout)")
	return cmd
}

func newFilePreviewCmd() *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "preview <id>",
		Short: "Download a file's preview image",
		Long: `Download the preview rendering of a file (GET /3.0/files/{id}/preview).
Like download this returns raw bytes, typically a PNG.`,
		Example: `  bexio file preview 12 --out preview.png`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fileWriteContent(cmd, args[0], out, true)
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "output file (default: name from the API, - for stdout)")
	return cmd
}

// fileWriteContent downloads a file (or its preview) and writes the bytes to
// out, "-" meaning stdout.
func fileWriteContent(cmd *cobra.Command, arg, out string, preview bool) error {
	id, err := parseID("file", arg)
	if err != nil {
		return err
	}
	client, err := newClient()
	if err != nil {
		return err
	}
	var content *api.FileContent
	if preview {
		content, err = client.PreviewFile(cmd.Context(), id)
	} else {
		content, err = client.DownloadFile(cmd.Context(), id)
	}
	if err != nil {
		return err
	}
	if out == "-" {
		_, err := cmd.OutOrStdout().Write(content.Data)
		return err
	}
	path := out
	if path == "" {
		path = content.Name
	}
	if path == "" {
		// The metadata endpoint knows the stored name; the download
		// response does not always carry a Content-Disposition.
		if f, err := client.GetFile(cmd.Context(), id); err == nil {
			path = fileNameWithExtension(f.Name, f.Extension)
		}
	}
	if path == "" {
		path = fmt.Sprintf("file-%d", id)
	}
	if preview && filepath.Ext(path) == "" {
		path += fileExtensionForMime(content.ContentType)
	}
	if err := os.WriteFile(path, content.Data, 0o644); err != nil { //nolint:gosec // downloads are not secrets
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s (%d bytes)\n", path, len(content.Data))
	return nil
}

// fileNameWithExtension appends the API extension when the name has none.
func fileNameWithExtension(name, ext string) string {
	if name == "" || ext == "" || strings.EqualFold(filepath.Ext(name), "."+ext) {
		return name
	}
	return name + "." + ext
}

func fileExtensionForMime(mimeType string) string {
	mimeType, _, _ = strings.Cut(mimeType, ";")
	switch strings.TrimSpace(mimeType) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "application/pdf":
		return ".pdf"
	default:
		return ""
	}
}

func newFileUsageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "usage <id>",
		Short: "Show the documents a file is attached to",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("file", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			refs, err := client.FileUsage(cmd.Context(), id)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				raws := make([]json.RawMessage, len(refs))
				for i, r := range refs {
					raws[i] = r.Raw
				}
				return output.JSON(cmd.OutOrStdout(), raws)
			}
			rows := make([][]string, len(refs))
			for i, r := range refs {
				rows[i] = []string{
					strconv.Itoa(r.ID),
					r.RefClass,
					r.DocumentNr,
					output.Truncate(r.Title, 40),
				}
			}
			output.Table(cmd.OutOrStdout(), []string{"id", "ref_class", "document_nr", "title"}, rows)
			return nil
		},
	}
}
