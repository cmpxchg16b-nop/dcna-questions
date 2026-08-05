package userupload

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"
)

// Errors returned by OnMemoryUserUploadManager operations.
var (
	// ErrEmptyFilename is returned by CreateNewUserUpload when the supplied
	// metadata has an empty Filename.
	ErrEmptyFilename = errors.New("userupload: filename must not be empty")

	// ErrUploadNotFound is returned by GetUserUploadByUploadId or
	// DeleteUserUpload when no upload exists for the given userId/uploadId
	// pair.
	ErrUploadNotFound = errors.New("userupload: upload not found")
)

// Compile-time check that OnMemoryUserUploadManager satisfies UserUploadManager.
var _ UserUploadManager = (*OnMemoryUserUploadManager)(nil)

// storedBlob is the value held in OnMemoryUserUploadManager.blobs.
type storedBlob struct {
	content []byte
	summary UserUploadSummery
}

// OnMemoryUserUploadManager is an in-memory, lock-free implementation of
// UserUploadManager. It is safe for concurrent use by multiple goroutines.
//
// It uses two sync.Maps:
//
//   - blobs maps the synthesized key "{userId}:{uploadId}" to a storedBlob;
//   - counts maps userId to an int64 holding the number of uploads ever
//     created for that user.
//
// CreateNewUserUpload claims a unique per-user index by compare-and-swapping
// the count (same technique as OnMemoryExamTrackingServer): it reads the
// current count c, then CAS-advances it to c+1; only when the CAS succeeds is
// index c known to be safe to use as the uploadId. Because the count is
// monotonically increased only by CreateNewUserUpload (and never decreased,
// even on DeleteUserUpload), there is no ABA problem: uploadIds are never
// reused.
//
// This implementation is diligent: CreateNewUserUpload reads the entire
// content of r into memory to compute the exact SizeBytes and the SHA-256
// digest (overriding any values supplied in metadata), auto-fills
// LastModifiedAt with the current time when it is zero, and rejects uploads
// with an empty filename.
type OnMemoryUserUploadManager struct {
	// blobs maps "{userId}:{uploadId}" to storedBlob.
	blobs sync.Map
	// counts maps userId to int64, the number of uploads ever created for
	// that user. Monotonically increasing; not decremented on delete.
	counts sync.Map
}

// NewOnMemoryUserUploadManager returns a ready-to-use OnMemoryUserUploadManager.
func NewOnMemoryUserUploadManager() *OnMemoryUserUploadManager {
	return &OnMemoryUserUploadManager{}
}

// CreateNewUserUpload stores a new upload owned by userId.
//
// It is diligent about the metadata:
//
//   - The entire content of r is read into memory; the exact byte count
//     overrides metadata.SizeBytes and the SHA-256 digest overrides
//     metadata.Sha256, regardless of what the caller supplied.
//   - When metadata.LastModifiedAt is zero it is auto-filled with the current
//     Unix millisecond timestamp.
//   - An upload with an empty metadata.Filename is rejected with
//     ErrEmptyFilename.
func (m *OnMemoryUserUploadManager) CreateNewUserUpload(_ context.Context, r io.Reader, userId string, metadata FileMetadata) (UserUploadSummery, error) {
	if metadata.Filename == "" {
		return UserUploadSummery{}, ErrEmptyFilename
	}

	// Read the entire upload to compute its exact size and digest.
	content, err := io.ReadAll(r)
	if err != nil {
		return UserUploadSummery{}, fmt.Errorf("userupload: reading upload content: %w", err)
	}

	// Always override SizeBytes with the actual size.
	metadata.SizeBytes = int64(len(content))

	// Always override Sha256 with the digest computed from the actual bytes,
	// encoded as lowercase hex.
	sum := sha256.Sum256(content)
	metadata.Sha256 = hex.EncodeToString(sum[:])

	// Auto-generate LastModifiedAt when omitted.
	if metadata.LastModifiedAt == 0 {
		metadata.LastModifiedAt = time.Now().UnixMilli()
	}

	// Claim a unique per-user index via CAS; the index becomes the uploadId.
	m.counts.LoadOrStore(userId, int64(0))
	var uploadId string
	for {
		cur, _ := m.counts.Load(userId)
		idx := cur.(int64)
		if m.counts.CompareAndSwap(userId, idx, idx+1) {
			uploadId = strconv.FormatInt(idx, 10)
			break
		}
	}

	summary := UserUploadSummery{
		UploadId:     uploadId,
		FileMetadata: metadata,
		UserId:       userId,
	}

	m.blobs.Store(blobKey(userId, uploadId), storedBlob{
		content: content,
		summary: summary,
	})

	return summary, nil
}

// ListUserUploads returns the summaries of all uploads owned by userId in the
// order they were created, or an empty slice if the user has none.
//
// Deleted uploads (whose blob has been removed but whose index is still
// counted) are skipped. The returned slice is independent of the stored state.
func (m *OnMemoryUserUploadManager) ListUserUploads(_ context.Context, userId string) ([]UserUploadSummery, error) {
	v, ok := m.counts.Load(userId)
	if !ok {
		return nil, nil
	}
	n := v.(int64)
	out := make([]UserUploadSummery, 0, n)
	for i := int64(0); i < n; i++ {
		uploadId := strconv.FormatInt(i, 10)
		if bv, ok := m.blobs.Load(blobKey(userId, uploadId)); ok {
			out = append(out, bv.(storedBlob).summary)
		}
	}
	return out, nil
}

// GetUserUploadByUploadId returns the summary of the upload identified by
// uploadId along with an io.ReadCloser over a fresh reader of its content.
// The returned ReadCloser must be closed by the caller when done; closing it
// does not affect the stored blob.
//
// The underlying byte slice is shared across readers; bytes.Reader never
// mutates it, so concurrent reads are safe.
func (m *OnMemoryUserUploadManager) GetUserUploadByUploadId(_ context.Context, userId string, uploadId string) (UserUploadSummery, io.ReadCloser, error) {
	v, ok := m.blobs.Load(blobKey(userId, uploadId))
	if !ok {
		return UserUploadSummery{}, nil, ErrUploadNotFound
	}
	blob := v.(storedBlob)
	return blob.summary, io.NopCloser(bytes.NewReader(blob.content)), nil
}

// DeleteUserUpload permanently removes the upload identified by uploadId from
// userId's uploads. The per-user count is not decremented, so uploadIds are
// never reused. It returns ErrUploadNotFound if no such upload exists.
func (m *OnMemoryUserUploadManager) DeleteUserUpload(_ context.Context, userId string, uploadId string) error {
	_, ok := m.blobs.LoadAndDelete(blobKey(userId, uploadId))
	if !ok {
		return ErrUploadNotFound
	}
	return nil
}

// blobKey builds the synthesized blobs-map key "{userId}:{uploadId}".
func blobKey(userId, uploadId string) string {
	return userId + ":" + uploadId
}
