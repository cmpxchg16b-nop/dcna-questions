package userupload

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// sha256Hex computes the lowercase hex SHA-256 of b, matching the encoding used
// by OnMemoryUserUploadManager.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestCreateAndGet_RoundTrip(t *testing.T) {
	mgr := NewOnMemoryUserUploadManager()
	ctx := context.Background()

	content := []byte("hello world")
	summary, err := mgr.CreateNewUserUpload(ctx, bytes.NewReader(content), "alice", FileMetadata{
		Filename:       "greeting.txt",
		MIMEType:       "text/plain",
		SizeBytes:      999, // should be overridden
		Sha256:         "deadbeef", // should be overridden
		LastModifiedAt: 1700000000000,
	})
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}

	if summary.UploadId == "" {
		t.Fatal("UploadId is empty")
	}
	if summary.UserId != "alice" {
		t.Errorf("UserId = %q, want alice", summary.UserId)
	}
	if summary.Filename != "greeting.txt" {
		t.Errorf("Filename = %q, want greeting.txt", summary.Filename)
	}
	if summary.MIMEType != "text/plain" {
		t.Errorf("MIMEType = %q, want text/plain", summary.MIMEType)
	}
	if summary.SizeBytes != int64(len(content)) {
		t.Errorf("SizeBytes = %d, want %d (must be overridden)", summary.SizeBytes, len(content))
	}
	if summary.LastModifiedAt != 1700000000000 {
		t.Errorf("LastModifiedAt = %d, want 1700000000000", summary.LastModifiedAt)
	}
	if want := sha256Hex(content); summary.Sha256 != want {
		t.Errorf("Sha256 = %q, want %q (must be overridden)", summary.Sha256, want)
	}

	// Retrieve content.
	gotSummary, rc, err := mgr.GetUserUploadByUploadId(ctx, "alice", summary.UploadId)
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	defer rc.Close()

	gotContent, _ := io.ReadAll(rc)
	if !bytes.Equal(gotContent, content) {
		t.Errorf("content = %q, want %q", gotContent, content)
	}
	if gotSummary != summary {
		t.Errorf("summary mismatch: got %+v, want %+v", gotSummary, summary)
	}
}

func TestCreate_OverridesSizeBytes(t *testing.T) {
	mgr := NewOnMemoryUserUploadManager()
	ctx := context.Background()

	content := make([]byte, 100)
	summary, err := mgr.CreateNewUserUpload(ctx, bytes.NewReader(content), "u", FileMetadata{
		Filename:  "f.bin",
		SizeBytes: 1, // wrong on purpose
	})
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}
	if summary.SizeBytes != 100 {
		t.Errorf("SizeBytes = %d, want 100 (must be overridden)", summary.SizeBytes)
	}
}

func TestCreate_OverridesSha256(t *testing.T) {
	mgr := NewOnMemoryUserUploadManager()
	ctx := context.Background()

	content := []byte("the real content")
	summary, err := mgr.CreateNewUserUpload(ctx, bytes.NewReader(content), "u", FileMetadata{
		Filename: "f.txt",
		Sha256:   "bogus-digest-from-client", // must be ignored
	})
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}
	if want := sha256Hex(content); summary.Sha256 != want {
		t.Errorf("Sha256 = %q, want %q (must be overridden)", summary.Sha256, want)
	}
}

func TestCreate_AutoFillsSha256WhenOmitted(t *testing.T) {
	mgr := NewOnMemoryUserUploadManager()
	ctx := context.Background()

	content := []byte("autofill me")
	summary, err := mgr.CreateNewUserUpload(ctx, bytes.NewReader(content), "u", FileMetadata{
		Filename: "f.txt",
		// Sha256 omitted
	})
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}
	if want := sha256Hex(content); summary.Sha256 != want {
		t.Errorf("Sha256 = %q, want %q", summary.Sha256, want)
	}
	if summary.Sha256 == "" {
		t.Error("Sha256 is empty, want a computed digest")
	}
}

func TestCreate_AutoFillsLastModifiedAt(t *testing.T) {
	mgr := NewOnMemoryUserUploadManager()
	ctx := context.Background()

	before := time.Now().UnixMilli()
	summary, err := mgr.CreateNewUserUpload(ctx, strings.NewReader("x"), "u", FileMetadata{
		Filename: "f.txt",
		// LastModifiedAt omitted
	})
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}
	after := time.Now().UnixMilli()

	if summary.LastModifiedAt < before || summary.LastModifiedAt > after {
		t.Errorf("LastModifiedAt = %d, want a value in [%d, %d]", summary.LastModifiedAt, before, after)
	}
}

func TestCreate_RejectsEmptyFilename(t *testing.T) {
	mgr := NewOnMemoryUserUploadManager()
	ctx := context.Background()

	_, err := mgr.CreateNewUserUpload(ctx, strings.NewReader("x"), "u", FileMetadata{
		Filename: "",
	})
	if !errors.Is(err, ErrEmptyFilename) {
		t.Fatalf("err = %v, want ErrEmptyFilename", err)
	}
}

func TestList_ReturnsUploadsInOrder(t *testing.T) {
	mgr := NewOnMemoryUserUploadManager()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, err := mgr.CreateNewUserUpload(ctx, strings.NewReader("x"), "alice", FileMetadata{
			Filename: "f.txt",
		})
		if err != nil {
			t.Fatalf("Create %d: unexpected error: %v", i, err)
		}
	}

	uploads, err := mgr.ListUserUploads(ctx, "alice")
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(uploads) != 3 {
		t.Fatalf("len(uploads) = %d, want 3", len(uploads))
	}
	// uploadIds should be "0", "1", "2" in order.
	for i, u := range uploads {
		want := string(rune('0' + i))
		if u.UploadId != want {
			t.Errorf("uploads[%d].UploadId = %q, want %q", i, u.UploadId, want)
		}
	}
}

func TestList_UnknownUserReturnsNil(t *testing.T) {
	mgr := NewOnMemoryUserUploadManager()
	ctx := context.Background()

	got, err := mgr.ListUserUploads(ctx, "nobody")
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("List unknown user = %v, want nil", got)
	}
}

func TestDelete_RemovesUpload(t *testing.T) {
	mgr := NewOnMemoryUserUploadManager()
	ctx := context.Background()

	summary, _ := mgr.CreateNewUserUpload(ctx, strings.NewReader("x"), "alice", FileMetadata{
		Filename: "f.txt",
	})

	if err := mgr.DeleteUserUpload(ctx, "alice", summary.UploadId); err != nil {
		t.Fatalf("Delete: unexpected error: %v", err)
	}

	_, _, err := mgr.GetUserUploadByUploadId(ctx, "alice", summary.UploadId)
	if !errors.Is(err, ErrUploadNotFound) {
		t.Fatalf("Get after delete: err = %v, want ErrUploadNotFound", err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	mgr := NewOnMemoryUserUploadManager()
	ctx := context.Background()

	err := mgr.DeleteUserUpload(ctx, "alice", "99")
	if !errors.Is(err, ErrUploadNotFound) {
		t.Fatalf("Delete: err = %v, want ErrUploadNotFound", err)
	}
}

func TestUsersAreIsolated(t *testing.T) {
	mgr := NewOnMemoryUserUploadManager()
	ctx := context.Background()

	// Alice creates two uploads (ids "0", "1"); Bob creates one (id "0").
	// Because uploadIds are scoped per user, both users have an upload with
	// id "0" but they must refer to different blobs.
	_, _ = mgr.CreateNewUserUpload(ctx, strings.NewReader("alice-0"), "alice", FileMetadata{
		Filename: "a0.txt",
	})
	_, _ = mgr.CreateNewUserUpload(ctx, strings.NewReader("alice-1"), "alice", FileMetadata{
		Filename: "a1.txt",
	})
	_, _ = mgr.CreateNewUserUpload(ctx, strings.NewReader("bob-0"), "bob", FileMetadata{
		Filename: "b0.txt",
	})

	// Bob cannot reach Alice's second upload (id "1").
	_, _, err := mgr.GetUserUploadByUploadId(ctx, "bob", "1")
	if !errors.Is(err, ErrUploadNotFound) {
		t.Fatalf("bob accessing alice-only upload id 1: err = %v, want ErrUploadNotFound", err)
	}

	// Bob's upload "0" returns Bob's content, not Alice's.
	bobSummary, rc, err := mgr.GetUserUploadByUploadId(ctx, "bob", "0")
	if err != nil {
		t.Fatalf("bob get own upload: unexpected error: %v", err)
	}
	defer rc.Close()
	if bobSummary.Filename != "b0.txt" {
		t.Errorf("bob upload 0 Filename = %q, want b0.txt", bobSummary.Filename)
	}
	body, _ := io.ReadAll(rc)
	if string(body) != "bob-0" {
		t.Errorf("bob upload 0 content = %q, want %q", body, "bob-0")
	}

	aliceUploads, _ := mgr.ListUserUploads(ctx, "alice")
	bobUploads, _ := mgr.ListUserUploads(ctx, "bob")
	if len(aliceUploads) != 2 || len(bobUploads) != 1 {
		t.Errorf("alice=%d, bob=%d, want 2 and 1", len(aliceUploads), len(bobUploads))
	}
}

func TestCreate_ConcurrentSameUserNoLoss(t *testing.T) {
	mgr := NewOnMemoryUserUploadManager()
	ctx := context.Background()

	const goroutines = 64
	const perGoroutine = 25
	const total = goroutines * perGoroutine
	userId := "racer"

	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := make(chan struct{})
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < perGoroutine; i++ {
				_, err := mgr.CreateNewUserUpload(ctx, strings.NewReader("x"), userId, FileMetadata{
					Filename: "f.txt",
				})
				if err != nil {
					t.Errorf("Create: unexpected error: %v", err)
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()

	uploads, err := mgr.ListUserUploads(ctx, userId)
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(uploads) != total {
		t.Fatalf("len(uploads) = %d, want %d (lost %d uploads)",
			len(uploads), total, total-len(uploads))
	}

	// Verify all uploadIds are unique.
	seen := make(map[string]bool, total)
	for _, u := range uploads {
		if seen[u.UploadId] {
			t.Fatalf("duplicate UploadId %q", u.UploadId)
		}
		seen[u.UploadId] = true
	}
}
