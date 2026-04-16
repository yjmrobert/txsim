package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yjmrobert/txsim/internal/store"
)

// TestSaveFailsWhenDirNotWritable drives the Save error branches: if the
// containing directory is read-only, CreateTemp fails and Save must return
// a wrapped error.
func TestSaveFailsWhenDirNotWritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses file permissions")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s, err := store.Open(path)
	require.NoError(t, err)

	// Strip write permission from the directory. CreateTemp then fails
	// inside Save.
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	err = s.Save(store.NewState())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create temp state")
}

// TestResetErrorsOnUnreadableDir exercises the Reset non-ErrNotExist branch.
func TestResetErrorsOnUnreadableDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses file permissions")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s, err := store.Open(path)
	require.NoError(t, err)
	require.NoError(t, s.Save(store.NewState()))

	// Make the parent dir non-writable so os.Remove on the child returns
	// EACCES rather than ENOENT.
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	err = s.Reset()
	require.Error(t, err)
	// Any error shape other than ErrNotExist is fine — the point is that
	// Reset propagates the error instead of swallowing it.
	assert.False(t, errors.Is(err, os.ErrNotExist))
}
