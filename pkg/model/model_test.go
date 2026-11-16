package model

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func softDeletedResource(t *testing.T) (*gorm.DB, KubernetesResource) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(&KubernetesResource{}); err != nil {
		t.Fatalf("migrate KubernetesResource: %v", err)
	}

	resource := KubernetesResource{
		ID:         "resource-1",
		APIVersion: "v1",
		Kind:       "Pod",
		ClusterID:  "cluster-1",
	}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatalf("create KubernetesResource: %v", err)
	}
	if err := db.Delete(&resource).Error; err != nil {
		t.Fatalf("soft-delete KubernetesResource: %v", err)
	}
	if !resource.DeletedAt.Valid {
		t.Fatal("expected Delete to set the tombstone timestamp")
	}
	return db, resource
}

func TestKubernetesResourceSoftDeleteExcludesFromNormalQueries(t *testing.T) {
	db, resource := softDeletedResource(t)

	var got KubernetesResource
	err := db.First(&got, "id = ?", resource.ID).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected gorm.ErrRecordNotFound, got %v", err)
	}
}

func TestKubernetesResourceSoftDeleteRetainsForUnscopedQueries(t *testing.T) {
	db, resource := softDeletedResource(t)

	var got KubernetesResource
	if err := db.Unscoped().First(&got, "id = ?", resource.ID).Error; err != nil {
		t.Fatalf("find tombstoned KubernetesResource: %v", err)
	}
	if !got.DeletedAt.Valid {
		t.Fatal("expected DeletedAt to contain the tombstone timestamp")
	}
}

func TestKubernetesResourceSoftDeleteOmitsFromWireRepresentation(t *testing.T) {
	_, resource := softDeletedResource(t)

	encoded, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("marshal KubernetesResource: %v", err)
	}
	if strings.Contains(string(encoded), "DeletedAt") || strings.Contains(string(encoded), "deletedAt") {
		t.Fatalf("DeletedAt leaked into JSON: %s", encoded)
	}
}
