package service

import (
	"context"
	"log/slog"
	"testing"

	"github.com/tabloy/keygate/internal/model"
)

// TestWebhookService_NilReceiver verifies that calling Dispatch or DispatchWithLog
// on a nil *WebhookService is completely safe and does not panic.
func TestWebhookService_NilReceiver(t *testing.T) {
	var s *WebhookService

	ctx := context.Background()
	payload := map[string]any{"license_id": "lic_123", "email": "test@example.com"}

	// Dispatch should safely no-op without panicking.
	s.Dispatch(ctx, "prod_123", "license.expired", payload)

	// DispatchWithLog should return nil error without panicking.
	if err := s.DispatchWithLog(ctx, "prod_123", "license.expired", payload); err != nil {
		t.Fatalf("expected nil error on nil WebhookService, got: %v", err)
	}
}

// TestExpiryChecker_NilWebhookAndEmail verifies that ExpiryChecker sweeps complete
// without panicking even when webhook and email services are nil.
func TestExpiryChecker_NilWebhookAndEmail(t *testing.T) {
	f := newFallbackFixture(t)
	c := NewExpiryChecker(f.s, nil, nil, slog.Default())

	plan := f.paidPlan(1)
	past := f.licenseIn(plan, model.StatusActive, ago(2*day))

	// Grace period expiry sweep must not panic on nil webhook/email.
	c.ExpireGracePeriodLicenses(f.ctx)
	if got := f.load(past).Status; got != model.StatusExpired {
		t.Fatalf("license status = %s, want %s", got, model.StatusExpired)
	}

	// Trial expiry sweep must not panic on nil webhook/email.
	trialPlan := f.plan("TrialPlan", "trial", "")
	trialLic := f.licenseIn(trialPlan, model.StatusTrialing, ago(15*day))
	c.ExpireTrials(f.ctx)
	if got := f.load(trialLic).Status; got != model.StatusExpired {
		t.Fatalf("trial license status = %s, want %s", got, model.StatusExpired)
	}

	// Past due expiry sweep must not panic on nil webhook/email.
	pastDueLic := f.licenseIn(plan, model.StatusPastDue, ago(35*day))
	if _, err := f.s.DB.NewRaw("UPDATE licenses SET past_due_at = ? WHERE id = ?", ago(35*day), pastDueLic.ID).Exec(f.ctx); err != nil {
		t.Fatal(err)
	}
	c.MarkPastDueAsExpired(f.ctx)
	if got := f.load(pastDueLic).Status; got != model.StatusExpired {
		t.Fatalf("past due license status = %s, want %s", got, model.StatusExpired)
	}

	// Leftover seat removals must not panic on nil webhook.
	c.FinishSeatRemovals(f.ctx)

	// Reminders with nil email must safely no-op.
	c.SendUpdatesEndingReminders(f.ctx)
	c.SendRenewalReminders(f.ctx)
}

// TestSeatService_NilWebhook verifies that SeatService operations (add, remove)
// complete without panicking when WebhookService is nil.
func TestSeatService_NilWebhook(t *testing.T) {
	f := newFallbackFixture(t)
	prod := &model.Product{Name: "Seats NilWH", Slug: "seats-nilwh-" + f.suffix, Type: "saas"}
	if err := f.s.CreateProduct(f.ctx, prod); err != nil {
		t.Fatal(err)
	}
	plan := &model.Plan{
		ProductID: prod.ID, Name: "SeatPlan", Slug: "seat-plan-" + f.suffix,
		LicenseType: "perpetual", LicenseModel: "standard", MaxSeats: 5, Active: true,
	}
	if err := f.s.CreatePlan(f.ctx, plan); err != nil {
		t.Fatal(err)
	}
	lic := &model.License{
		ProductID: prod.ID, PlanID: plan.ID, Email: "owner-nilwh-" + f.suffix + "@example.com",
		LicenseKey: "KG-NILWH-" + f.suffix, Status: model.StatusActive,
	}
	if err := f.s.CreateLicense(f.ctx, lic); err != nil {
		t.Fatal(err)
	}

	seatSvc := NewSeatService(f.s, nil, nil, slog.Default(), "")

	seat, err := seatSvc.AddSeat(f.ctx, AddSeatInput{
		LicenseKey: lic.LicenseKey,
		Email:      "member-nilwh-" + f.suffix + "@example.com",
		Role:       "member",
		ProductID:  prod.ID,
		ActorEmail: lic.Email,
	})
	if err != nil {
		t.Fatalf("AddSeat failed: %v", err)
	}

	if err := seatSvc.RemoveSeat(f.ctx, lic.LicenseKey, seat.ID, prod.ID, "user_actor", lic.Email); err != nil {
		t.Fatalf("RemoveSeat failed: %v", err)
	}
}

// TestUsageService_NilWebhook verifies that UsageService quota events do not panic
// when WebhookService is nil.
func TestUsageService_NilWebhook(t *testing.T) {
	f := newFallbackFixture(t)
	usageSvc := NewUsageService(f.s, nil, nil, slog.Default(), 0.8)

	// Create plan with quota entitlement
	plan := f.plan("QuotaPlan", "subscription", "")
	ent := &model.Entitlement{
		ID:          "ent_" + f.suffix,
		PlanID:      plan.ID,
		Feature:     "api_calls",
		ValueType:   "quota",
		Value:       "10",
		QuotaPeriod: "monthly",
	}
	if err := f.s.CreateEntitlement(f.ctx, ent); err != nil {
		t.Fatal(err)
	}

	lic := f.licenseIn(plan, model.StatusActive, nil)

	// 1. Trigger quota warning threshold (8 / 10 = 80%) with nil webhook
	_, err := usageSvc.RecordUsage(f.ctx, RecordUsageInput{
		LicenseKey: lic.LicenseKey,
		Feature:    "api_calls",
		Quantity:   8,
		ProductID:  f.prod.ID,
	})
	if err != nil {
		t.Fatalf("RecordUsage under quota failed: %v", err)
	}

	// 2. Trigger quota exceeded (8 + 5 = 13 > 10) with nil webhook
	_, err = usageSvc.RecordUsage(f.ctx, RecordUsageInput{
		LicenseKey: lic.LicenseKey,
		Feature:    "api_calls",
		Quantity:   5,
		ProductID:  f.prod.ID,
	})
	if err == nil {
		t.Fatalf("expected QUOTA_EXCEEDED error, got nil")
	}
}
