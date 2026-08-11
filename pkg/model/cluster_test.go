package model

import (
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupClusterTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&Cluster{}, &User{}, &Role{}, &RoleAssignment{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	DB = db
}

// --- UUID auto-generation tests ---

func TestAddCluster_AutoGeneratesUUID(t *testing.T) {
	setupClusterTestDB(t)

	cluster := &Cluster{Name: "test-cluster"}
	if err := AddCluster(cluster); err != nil {
		t.Fatalf("AddCluster() error: %v", err)
	}
	if cluster.UUID == "" {
		t.Fatal("AddCluster() should auto-generate UUID")
	}
	if _, err := uuid.Parse(cluster.UUID); err != nil {
		t.Errorf("UUID %q is not a valid UUID: %v", cluster.UUID, err)
	}
}

func TestAddCluster_PreservesPresetUUID(t *testing.T) {
	setupClusterTestDB(t)

	preset := "550e8400-e29b-41d4-a716-446655440000"
	cluster := &Cluster{Name: "test-cluster", UUID: preset}
	if err := AddCluster(cluster); err != nil {
		t.Fatalf("AddCluster() error: %v", err)
	}
	if cluster.UUID != preset {
		t.Errorf("AddCluster() overwrote preset UUID: got %q, want %q", cluster.UUID, preset)
	}
}

func TestAddCluster_GeneratesUniqueUUIDs(t *testing.T) {
	setupClusterTestDB(t)

	c1 := &Cluster{Name: "cluster-1"}
	c2 := &Cluster{Name: "cluster-2"}
	if err := AddCluster(c1); err != nil {
		t.Fatalf("AddCluster(c1) error: %v", err)
	}
	if err := AddCluster(c2); err != nil {
		t.Fatalf("AddCluster(c2) error: %v", err)
	}
	if c1.UUID == c2.UUID {
		t.Fatal("two clusters should have different UUIDs")
	}
}

// --- GetClusterByUUID tests ---

func TestGetClusterByUUID_Found(t *testing.T) {
	setupClusterTestDB(t)

	cluster := &Cluster{Name: "find-me"}
	if err := AddCluster(cluster); err != nil {
		t.Fatalf("AddCluster() error: %v", err)
	}

	got, err := GetClusterByUUID(cluster.UUID)
	if err != nil {
		t.Fatalf("GetClusterByUUID() error: %v", err)
	}
	if got.Name != "find-me" {
		t.Errorf("GetClusterByUUID() returned cluster %q, want %q", got.Name, "find-me")
	}
	if got.ID != cluster.ID {
		t.Errorf("GetClusterByUUID() returned ID %d, want %d", got.ID, cluster.ID)
	}
}

func TestGetClusterByUUID_NotFound(t *testing.T) {
	setupClusterTestDB(t)

	_, err := GetClusterByUUID("nonexistent-uuid")
	if err == nil {
		t.Fatal("GetClusterByUUID() should return error for nonexistent UUID")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetClusterByUUID() error = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestGetClusterByUUID_EmptyString(t *testing.T) {
	setupClusterTestDB(t)

	_, err := GetClusterByUUID("")
	if err == nil {
		t.Fatal("GetClusterByUUID() should return error for empty UUID")
	}
}

// --- GetClusterByConnectorTokenHash tests ---

func TestGetClusterByConnectorTokenHash(t *testing.T) {
	setupClusterTestDB(t)

	hash := "abcdef0123456789"
	cluster := &Cluster{
		Name:               "token-cluster",
		Connector:          true,
		ConnectorTokenHash: hash,
	}
	if err := AddCluster(cluster); err != nil {
		t.Fatalf("AddCluster() error: %v", err)
	}

	got, err := GetClusterByConnectorTokenHash(hash)
	if err != nil {
		t.Fatalf("GetClusterByConnectorTokenHash() error: %v", err)
	}
	if got.Name != "token-cluster" {
		t.Errorf("got cluster %q, want %q", got.Name, "token-cluster")
	}
}

func TestGetClusterByConnectorTokenHash_NotFound(t *testing.T) {
	setupClusterTestDB(t)

	_, err := GetClusterByConnectorTokenHash("nonexistent-hash")
	if err == nil {
		t.Fatal("GetClusterByConnectorTokenHash() should return error for nonexistent hash")
	}
}

// --- migrateClusterUUIDs tests ---

func TestMigrateClusterUUIDs_BackfillsEmpty(t *testing.T) {
	setupClusterTestDB(t)

	// Insert a cluster directly with empty UUID (bypassing AddCluster)
	cluster := Cluster{Name: "old-cluster", UUID: ""}
	if err := DB.Create(&cluster).Error; err != nil {
		t.Fatalf("DB.Create() error: %v", err)
	}

	migrateClusterUUIDs()

	var refreshed Cluster
	if err := DB.First(&refreshed, cluster.ID).Error; err != nil {
		t.Fatalf("DB.First() error: %v", err)
	}
	if refreshed.UUID == "" {
		t.Fatal("migrateClusterUUIDs() should have backfilled the UUID")
	}
	if _, err := uuid.Parse(refreshed.UUID); err != nil {
		t.Errorf("backfilled UUID %q is not valid: %v", refreshed.UUID, err)
	}
}

func TestMigrateClusterUUIDs_PreservesExisting(t *testing.T) {
	setupClusterTestDB(t)

	existing := "550e8400-e29b-41d4-a716-446655440000"
	cluster := Cluster{Name: "has-uuid", UUID: existing}
	if err := DB.Create(&cluster).Error; err != nil {
		t.Fatalf("DB.Create() error: %v", err)
	}

	migrateClusterUUIDs()

	var refreshed Cluster
	if err := DB.First(&refreshed, cluster.ID).Error; err != nil {
		t.Fatalf("DB.First() error: %v", err)
	}
	if refreshed.UUID != existing {
		t.Errorf("migrateClusterUUIDs() changed existing UUID: got %q, want %q", refreshed.UUID, existing)
	}
}
