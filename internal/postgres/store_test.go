package postgres

import (
	"testing"
)

func TestMigrationsAreEmbedded(t *testing.T) {
	for _, file := range []string{
		"migrations/001_inventory_items.sql",
		"migrations/002_idempotency_requests.sql",
	} {
		if _, err := migrations.ReadFile(file); err != nil {
			t.Errorf("read embedded migration %q: %v", file, err)
		}
	}
}
