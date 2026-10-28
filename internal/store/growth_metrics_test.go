package store_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tabloy/keygate/internal/model"
	"github.com/tabloy/keygate/internal/store"
)

func TestGrowthMetricsTrialConversion(t *testing.T) {
	s := setupTestDB(t)
	defer s.Close()
	ctx := context.Background()

	suffix := strings.ReplaceAll(time.Now().Format("150405.000000"), ".", "")
	prod := &model.Product{Name: "Growth Prod", Slug: "growth-" + suffix, Type: "desktop"}
	if err := s.CreateProduct(ctx, prod); err != nil {
		t.Fatal(err)
	}

	trialPlan := &model.Plan{
		ProductID:    prod.ID,
		Name:         "Trial Plan",
		Slug:         "trial-" + suffix,
		LicenseType:  "trial",
		LicenseModel: "standard",
		TrialDays:    14,
		Active:       true,
	}
	if err := s.CreatePlan(ctx, trialPlan); err != nil {
		t.Fatal(err)
	}

	paidPlan := &model.Plan{
		ProductID:    prod.ID,
		Name:         "Paid Plan",
		Slug:         "paid-" + suffix,
		LicenseType:  "subscription",
		LicenseModel: "standard",
		Active:       true,
	}
	if err := s.CreatePlan(ctx, paidPlan); err != nil {
		t.Fatal(err)
	}

	lic := &model.License{
		ProductID:  prod.ID,
		PlanID:     trialPlan.ID,
		Email:      "trial-user-" + suffix + "@example.com",
		LicenseKey: "KG-TR-" + suffix,
		Status:     model.StatusTrialing,
	}
	if err := s.CreateLicense(ctx, lic); err != nil {
		t.Fatal(err)
	}

	// 1. While in trial: TotalTrials = 1, ConvertedTrials = 0, TrialConversion = 0%
	gm, err := s.GetGrowthMetrics(ctx, prod.ID)
	if err != nil {
		t.Fatalf("GetGrowthMetrics initial: %v", err)
	}
	if gm.TotalTrials != 1 {
		t.Errorf("TotalTrials = %d, want 1", gm.TotalTrials)
	}
	if gm.ConvertedTrials != 0 {
		t.Errorf("ConvertedTrials = %d, want 0", gm.ConvertedTrials)
	}
	if gm.TrialConversion != 0 {
		t.Errorf("TrialConversion = %f, want 0", gm.TrialConversion)
	}

	// 2. Simulate upgrade to paid plan
	upgrade := &model.LicenseUpgrade{
		LicenseID:               lic.ID,
		FromPlanID:              trialPlan.ID,
		ToPlanID:                paidPlan.ID,
		State:                   model.UpgradeApplied,
		StripeCheckoutSessionID: "cs_test_" + suffix,
	}
	if err := store.CreateLicenseUpgradeIn(ctx, s.DB, upgrade); err != nil {
		t.Fatalf("CreateLicenseUpgradeIn: %v", err)
	}

	lic.PlanID = paidPlan.ID
	lic.Status = model.StatusActive
	if err := s.UpdateLicense(ctx, lic, "plan_id", "status"); err != nil {
		t.Fatalf("UpdateLicense: %v", err)
	}

	// 3. After upgrade: TotalTrials = 1, ConvertedTrials = 1, TrialConversion = 100%
	gm, err = s.GetGrowthMetrics(ctx, prod.ID)
	if err != nil {
		t.Fatalf("GetGrowthMetrics after upgrade: %v", err)
	}
	if gm.TotalTrials != 1 {
		t.Errorf("TotalTrials after upgrade = %d, want 1", gm.TotalTrials)
	}
	if gm.ConvertedTrials != 1 {
		t.Errorf("ConvertedTrials after upgrade = %d, want 1", gm.ConvertedTrials)
	}
	if gm.TrialConversion != 100.0 {
		t.Errorf("TrialConversion after upgrade = %f, want 100.0", gm.TrialConversion)
	}
}
