// Package userupload defines the data model for files uploaded by users.
//
// Each upload belongs to exactly one authenticated user (no anonymous uploads
// are allowed) and corresponds to a single file: directories and multi-file
// uploads are not supported.
package userupload

import (
	"context"
	"io"
)

// FileMetadata describes the properties of a stored file. It is reused by
// types that need to carry file-level information without ownership details.
type FileMetadata struct {
	// Filename is the original filename supplied by the client at upload time.
	Filename string

	// MIMEType is the original MIME type of the upload, as detected or
	// declared at upload time. Most HTTP clients recognize it and can easily
	// attach it while re-uploading the file to another server.
	MIMEType string

	// SizeBytes is the size of the original uploaded file in bytes.
	SizeBytes int64

	// LastModifiedAt is the Unix millisecond timestamp at which the upload
	// happened.
	LastModifiedAt int64

	// Sha256 is the lowercase hex-encoded SHA-256 digest of the uploaded
	// file's content. A diligent UserUploadManager implementation computes it
	// from the actual bytes and overrides any caller-supplied value; a
	// non-diligent implementation may leave it empty.
	Sha256 string
}

// UserUploadSummery is the metadata describing a single user upload. It embeds
// FileMetadata for the file-level fields and adds the ownership/identity fields.
// It does not carry the file content itself; use the UserUploadManager to
// retrieve the content stream of an upload.
type UserUploadSummery struct {
	// UploadId is the implementation-defined identifier of this upload. It is
	// unique within the scope of the owning user and is used as the handle for
	// retrieving or deleting the upload via UserUploadManager.
	UploadId string

	FileMetadata

	// UserId is the id of the user who performed the upload. Anonymous
	// uploads are not allowed, so every upload has a non-empty UserId.
	UserId string
}

// UserUploadManager stores and retrieves user uploads.
//
// All operations are scoped to a user: uploads created by one user are not
// visible or mutable through the userid of another. Both userid and uploadId
// are opaque strings whose meaning is defined by the implementation.
type UserUploadManager interface {
	// CreateNewUserUpload stores a new upload owned by userId, reading the
	// file content from r. The metadata describes the file being uploaded
	// (filename, MIME type, size, and last-modified timestamp). It returns
	// the summary of the newly created upload. The caller is responsible for
	// closing r if appropriate.
	//
	// Note: when metadata.SizeBytes is left as zero, the implementation will
	// not attempt to compute or guarantee the precise size of the stored file,
	// and the SizeBytes field in the UserUploadSummery returned by subsequent
	// calls (such as ListUserUploads or GetUserUploadByUploadId) may remain
	// zero or be inexact. Callers that need an accurate size must supply it
	// upfront via metadata.SizeBytes.
	// In another word, an specific implementation might or might not be diligent.
	CreateNewUserUpload(ctx context.Context, r io.Reader, userId string, metadata FileMetadata) (UserUploadSummery, error)

	// ListUserUploads returns the summaries of all uploads owned by userId,
	// or an empty slice if the user has none.
	ListUserUploads(ctx context.Context, userId string) ([]UserUploadSummery, error)

	// GetUserUploadByUploadId returns the summary of the upload identified
	// by uploadId along with an io.ReadCloser over its content. The returned
	// ReadCloser must be closed by the caller when done.
	GetUserUploadByUploadId(ctx context.Context, userId string, uploadId string) (UserUploadSummery, io.ReadCloser, error)

	// DeleteUserUpload permanently removes the upload identified by uploadId
	// from userId's uploads. It returns an error if the upload does not exist
	// or does not belong to userId.
	DeleteUserUpload(ctx context.Context, userId string, uploadId string) error
}
