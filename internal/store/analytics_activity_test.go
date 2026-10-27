package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/tabloy/keygate/internal/model"
)

func TestGetRecentActivityScopesToProduct(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("skipping integration test: TEST_DATABASE_URL not set")
	}
	seed, err := New(dsn)
	if err != nil {
		t.Skipf("skipping integration test: %v", err)
	}
	defer seed.Close()
	if err := seed.RunMigrations("../../db/migrations"); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000")

	prodA := &model.Product{Name: "Prod A", Slug: "act-a-" + suffix, Type: "desktop"}
	if err := seed.CreateProduct(ctx, prodA); err != nil {
		t.Fatal(err)
	}
	prodB := &model.Product{Name: "Prod B", Slug: "act-b-" + suffix, Type: "desktop"}
	if err := seed.CreateProduct(ctx, prodB); err != nil {
		t.Fatal(err)
	}

	seed.Audit(ctx, &model.AuditLog{
		Entity:    "product",
		EntityID:  prodA.ID,
		Action:    "product.create",
		ActorType: "admin",
	})
	seed.Audit(ctx, &model.AuditLog{
		Entity:    "product",
		EntityID:  prodB.ID,
		Action:    "product.create",
		ActorType: "admin",
	})

	actsA, err := seed.GetRecentActivity(ctx, prodA.ID, 10)
	if err != nil {
		t.Fatalf("GetRecentActivity prodA: %v", err)
	}
	for _, a := range actsA {
		if a.EntityID == prodB.ID {
			t.Errorf("GetRecentActivity leaked prodB activity into prodA results: %+v", a)
		}
	}
}
