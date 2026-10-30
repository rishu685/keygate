package store_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tabloy/keygate/internal/model"
	"github.com/tabloy/keygate/internal/store"
)

// A customer's usage in Analytics Insights is what their licences
// recorded, however many devices those licences run on. Joining the
// devices row by row counted a licence's usage once per device, so a
// customer on three machines showed three times their usage and was
// ranked above one who had used more.
func TestTopUsersCountsUsageOncePerLicense(t *testing.T) {
	s := setupTestDB(t)
	t.Cleanup(func() { s.Close() })
	ctx := context.Background()

	suffix := strings.ReplaceAll(time.Now().Format("150405.000000"), ".", "")
	prod := &model.Product{Name: "Top users", Slug: "top-users-" + suffix, Type: "desktop"}
	if err := s.CreateProduct(ctx, prod); err != nil {
		t.Fatal(err)
	}
	plan := &model.Plan{ProductID: prod.ID, Name: "Pro", Slug: "top-users-pro-" + suffix, LicenseType: "perpetual", LicenseModel: "standard"}
	if err := s.CreatePlan(ctx, plan); err != nil {
		t.Fatal(err)
	}
	n := 0
	licence := func(email string, usage int64, devices int) {
		t.Helper()
		n++
		l := &model.License{ProductID: prod.ID, PlanID: plan.ID, Email: email,
			LicenseKey: "KG-TOP-" + suffix + "-" + string(rune('a'+n)), Status: model.StatusActive}
		if err := s.CreateLicense(ctx, l); err != nil {
			t.Fatal(err)
		}
		if err := s.RecordUsageEvent(ctx, &model.UsageEvent{LicenseID: l.ID, Feature: "exports", Quantity: usage}); err != nil {
			t.Fatal(err)
		}
		for i := range devices {
			if err := s.CreateActivation(ctx, &model.Activation{LicenseID: l.ID,
				Identifier: l.LicenseKey + "-device-" + string(rune('a'+i)), IdentifierType: "device"}); err != nil {
				t.Fatal(err)
			}
		}
	}
	many := "many-devices-" + suffix + "@example.com"
	heavy := "heavy-user-" + suffix + "@example.com"
	two := "two-licences-" + suffix + "@example.com"
	licence(many, 100, 3)
	licence(heavy, 150, 0)
	licence(two, 10, 2)
	licence(two, 5, 0)

	top, err := s.GetTopUsers(ctx, prod.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		licences, devices int
		usage             int64
	}
	got := map[string]row{}
	order := []string{}
	for _, u := range top {
		got[u.Email] = row{u.LicenseCount, u.ActivationCount, u.TotalUsage}
		order = append(order, u.Email)
	}
	for email, want := range map[string]row{
		many:  {1, 3, 100},
		heavy: {1, 0, 150},
		two:   {2, 2, 15},
	} {
		if got[email] != want {
			t.Errorf("%s: %+v, want %+v", email, got[email], want)
		}
	}
	// Two licences first, then by usage.
	if want := []string{two, heavy, many}; strings.Join(order, ",") != strings.Join(want, ",") {
		t.Errorf("ranking %v, want %v", order, want)
	}
}

// TestTopUsersAggregatesCustomerAcrossUserIDs verifies that licenses
// with and without user_id for the same customer email are consolidated
// into a single TopUser entry, with user_id populated if available.
func TestTopUsersAggregatesCustomerAcrossUserIDs(t *testing.T) {
	s := setupTestDB(t)
	t.Cleanup(func() { s.Close() })
	ctx := context.Background()

	suffix := strings.ReplaceAll(time.Now().Format("150405.000000"), ".", "")
	prod := &model.Product{Name: "Top Users Consolidation", Slug: "top-cons-" + suffix, Type: "desktop"}
	if err := s.CreateProduct(ctx, prod); err != nil {
		t.Fatal(err)
	}
	plan := &model.Plan{ProductID: prod.ID, Name: "Pro", Slug: "top-cons-pro-" + suffix, LicenseType: "perpetual", LicenseModel: "standard"}
	if err := s.CreatePlan(ctx, plan); err != nil {
		t.Fatal(err)
	}

	custEmail := "customer-" + suffix + "@example.com"
	userID := "usr_" + suffix

	// License 1: issued by admin, no user_id attached.
	l1 := &model.License{ProductID: prod.ID, PlanID: plan.ID, Email: custEmail,
		LicenseKey: "KG-CONS-" + suffix + "-1", Status: model.StatusActive}
	if err := s.CreateLicense(ctx, l1); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordUsageEvent(ctx, &model.UsageEvent{LicenseID: l1.ID, Feature: "compute", Quantity: 50}); err != nil {
		t.Fatal(err)
	}

	// User account created in portal.
	user := &model.User{ID: userID, Email: custEmail}
	if err := s.UpsertUser(ctx, user); err != nil {
		t.Fatal(err)
	}

	// License 2: issued after portal sign-in, user_id attached.
	l2 := &model.License{ProductID: prod.ID, PlanID: plan.ID, Email: custEmail, UserID: userID,
		LicenseKey: "KG-CONS-" + suffix + "-2", Status: model.StatusActive}
	if err := s.CreateLicense(ctx, l2); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordUsageEvent(ctx, &model.UsageEvent{LicenseID: l2.ID, Feature: "compute", Quantity: 75}); err != nil {
		t.Fatal(err)
	}

	top, err := s.GetTopUsers(ctx, prod.ID, 10)
	if err != nil {
		t.Fatal(err)
	}

	var found *store.TopUser
	foundCount := 0
	for _, u := range top {
		if strings.EqualFold(u.Email, custEmail) {
			foundCount++
			copy := u
			found = &copy
		}
	}
	if foundCount != 1 {
		t.Fatalf("expected 1 aggregated entry for customer %s, got %d", custEmail, foundCount)
	}
	if found.LicenseCount != 2 {
		t.Errorf("got license count %d, want 2", found.LicenseCount)
	}
	if found.TotalUsage != 125 {
		t.Errorf("got total usage %d, want 125", found.TotalUsage)
	}
	if found.UserID != userID {
		t.Errorf("got user_id %q, want %q", found.UserID, userID)
	}
}

// TestTopUsersExcludesMixedCaseAdmin verifies that admin/owner users
// with mixed-case emails are excluded from customer rankings.
func TestTopUsersExcludesMixedCaseAdmin(t *testing.T) {
	s := setupTestDB(t)
	t.Cleanup(func() { s.Close() })
	ctx := context.Background()

	suffix := strings.ReplaceAll(time.Now().Format("150405.000000"), ".", "")
	prod := &model.Product{Name: "Top Users Admin", Slug: "top-admin-" + suffix, Type: "desktop"}
	if err := s.CreateProduct(ctx, prod); err != nil {
		t.Fatal(err)
	}
	plan := &model.Plan{ProductID: prod.ID, Name: "Pro", Slug: "top-admin-pro-" + suffix, LicenseType: "perpetual", LicenseModel: "standard"}
	if err := s.CreatePlan(ctx, plan); err != nil {
		t.Fatal(err)
	}

	adminEmail := "Admin-" + suffix + "@Example.com"
	adminUser := &model.User{Email: adminEmail, Role: model.RoleAdmin}
	if err := s.UpsertUser(ctx, adminUser); err != nil {
		t.Fatal(err)
	}
	// Explicitly store mixed-case email and admin role on the user
	if _, err := s.DB.NewUpdate().Model((*model.User)(nil)).
		Set("email = ?, role = ?", adminEmail, model.RoleAdmin).
		Where("id = ?", adminUser.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}

	// Create license for admin
	adminLic := &model.License{ProductID: prod.ID, PlanID: plan.ID, Email: strings.ToLower(adminEmail),
		LicenseKey: "KG-ADM-" + suffix, Status: model.StatusActive}
	if err := s.CreateLicense(ctx, adminLic); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordUsageEvent(ctx, &model.UsageEvent{LicenseID: adminLic.ID, Feature: "api", Quantity: 500}); err != nil {
		t.Fatal(err)
	}

	// Create license for genuine customer
	custEmail := "real-customer-" + suffix + "@example.com"
	custLic := &model.License{ProductID: prod.ID, PlanID: plan.ID, Email: custEmail,
		LicenseKey: "KG-CUST-" + suffix, Status: model.StatusActive}
	if err := s.CreateLicense(ctx, custLic); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordUsageEvent(ctx, &model.UsageEvent{LicenseID: custLic.ID, Feature: "api", Quantity: 10}); err != nil {
		t.Fatal(err)
	}

	top, err := s.GetTopUsers(ctx, prod.ID, 10)
	if err != nil {
		t.Fatal(err)
	}

	for _, u := range top {
		if strings.EqualFold(u.Email, adminEmail) {
			t.Fatalf("admin user %s was not excluded from TopUsers", u.Email)
		}
	}
	if len(top) != 1 || !strings.EqualFold(top[0].Email, custEmail) {
		t.Fatalf("expected only real customer in TopUsers, got: %+v", top)
	}
}
