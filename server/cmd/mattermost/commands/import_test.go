// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package commands

import (
	"context"
	dbsql "database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// countUsersByUsername counts rows in the Users table for the given username,
// querying the same database the CLI subprocess wrote to.
func countUsersByUsername(t *testing.T, dsn, username string) int {
	t.Helper()

	db, err := dbsql.Open(model.DatabaseDriverPostgres, dsn)
	require.NoError(t, err)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM Users WHERE Username = $1", username).Scan(&count)
	require.NoError(t, err)

	return count
}

// writeUserImportFile writes a minimal JSONL import file containing a single user.
func writeUserImportFile(t *testing.T, username string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "import.jsonl")
	content := fmt.Sprintf(
		"{\"type\":\"version\",\"version\":1}\n"+
			"{\"type\":\"user\",\"user\":{\"username\":%q,\"email\":\"%s@import-test.local\"}}\n",
		username, username)
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	return path
}

// TestBulkImportApply covers the contract of the --apply flag: without it the
// import only validates, with it the data is written to the database.
//
// This is a regression test. The flag was silently ignored for an extended
// period because dryRun was passed positionally and got shifted when a new
// parameter was inserted into BulkImportWithPath. The command still printed
// its success messages and exited 0, so nothing surfaced the failure.
func TestBulkImportApply(t *testing.T) {
	if testing.Short() {
		t.Skip("requires live test database")
	}

	th := SetupWithStoreMock(t)
	th.SetAutoConfig(false)

	dsn := dsnFromHelper(t)

	t.Run("without --apply the user is not persisted", func(t *testing.T) {
		username := "importtest" + model.NewId()[:8]
		path := writeUserImportFile(t, username)

		output, err := th.RunCommandWithOutput(t, "--config", dsn, "import", "bulk", path)
		require.NoError(t, err, output)
		require.Contains(t, output, "Running Bulk Import Data Validation.")

		require.Equal(t, 0, countUsersByUsername(t, dsn, username),
			"validation-only run must not write to the database")
	})

	t.Run("with --apply the user is persisted", func(t *testing.T) {
		username := "importtest" + model.NewId()[:8]
		path := writeUserImportFile(t, username)

		output, err := th.RunCommandWithOutput(t, "--config", dsn, "import", "bulk", "--apply", path)
		require.NoError(t, err, output)
		require.Contains(t, output, "Finished Bulk Import.")

		require.Equal(t, 1, countUsersByUsername(t, dsn, username),
			"--apply must write to the database")
	})
}
