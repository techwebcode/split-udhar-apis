package services

import (
	"math"
	"testing"

	"split-udhar-apis/models"

	"gorm.io/gorm"
)

func sumDeltas(deltas map[string]float64) float64 {
	total := 0.0
	for _, d := range deltas {
		total += d
	}
	return total
}

func newGroup(mobiles ...string) *models.Group {
	group := &models.Group{}
	for _, m := range mobiles {
		group.Members = append(group.Members, models.GroupMember{UserMobile: m})
	}
	return group
}

// An expense only moves money between members, so the deltas must net to zero.
func TestExpenseBalanceDeltasSumToZero(t *testing.T) {
	cases := []struct {
		name   string
		payer  string
		split  []string
		amount float64
	}{
		{"payer in split", "A", []string{"A", "B", "C"}, 300},
		{"payer outside split", "A", []string{"B", "C"}, 300},
		{"single member", "A", []string{"A"}, 100},
		{"amount not evenly divisible", "A", []string{"A", "B", "C"}, 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deltas := expenseBalanceDeltas(tc.payer, tc.split, tc.amount, 1)
			if got := sumDeltas(deltas); math.Abs(got) > 1e-9 {
				t.Fatalf("deltas should net to zero, got %v (%v)", got, deltas)
			}
		})
	}
}

func TestExpenseBalanceDeltasPayerInSplit(t *testing.T) {
	// A pays 300 split three ways: A is owed the other two shares.
	deltas := expenseBalanceDeltas("A", []string{"A", "B", "C"}, 300, 1)

	if got, want := deltas["A"], 200.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("payer delta = %v, want %v", got, want)
	}
	for _, m := range []string{"B", "C"} {
		if got, want := deltas[m], -100.0; math.Abs(got-want) > 1e-9 {
			t.Errorf("member %s delta = %v, want %v", m, got, want)
		}
	}
}

// The pre-fix code credited the payer nothing when they were not part of the
// split, silently losing the money they laid out.
func TestExpenseBalanceDeltasPayerOutsideSplit(t *testing.T) {
	deltas := expenseBalanceDeltas("A", []string{"B", "C"}, 300, 1)

	if got, want := deltas["A"], 300.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("payer outside split should be credited in full, got %v want %v", got, want)
	}
	for _, m := range []string{"B", "C"} {
		if got, want := deltas[m], -150.0; math.Abs(got-want) > 1e-9 {
			t.Errorf("member %s delta = %v, want %v", m, got, want)
		}
	}
}

// Applying an expense then reverting it must leave every balance untouched.
func TestExpenseBalanceDeltasApplyThenRevertIsNoop(t *testing.T) {
	payer, split, amount := "A", []string{"A", "B", "C"}, 250.0

	net := map[string]float64{}
	for m, d := range expenseBalanceDeltas(payer, split, amount, 1) {
		net[m] += d
	}
	for m, d := range expenseBalanceDeltas(payer, split, amount, -1) {
		net[m] += d
	}

	for m, d := range net {
		if math.Abs(d) > 1e-9 {
			t.Errorf("member %s left with %v after apply+revert, want 0", m, d)
		}
	}
}

func TestExpenseBalanceDeltasIgnoresInvalidInput(t *testing.T) {
	if got := expenseBalanceDeltas("A", []string{"A", "B"}, 0, 1); len(got) != 0 {
		t.Errorf("zero amount should produce no deltas, got %v", got)
	}
	if got := expenseBalanceDeltas("A", []string{"A", "B"}, -50, 1); len(got) != 0 {
		t.Errorf("negative amount should produce no deltas, got %v", got)
	}
	if got := expenseBalanceDeltas("A", nil, 100, 1); len(got) != 0 {
		t.Errorf("empty split should produce no deltas, got %v", got)
	}
}

// This is the corruption the SplitWith column exists to prevent: an expense
// split across a subset must revert across that same subset.
func TestSplitMembersForUsesStoredSubset(t *testing.T) {
	group := newGroup("A", "B", "C", "D")
	expense := &models.GroupExpense{SplitWith: "A,B"}

	got := splitMembersFor(group, expense)
	if len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Fatalf("splitMembersFor = %v, want [A B]", got)
	}

	// Reverting must only touch A and B, leaving C and D alone.
	deltas := expenseBalanceDeltas(expense.PayerMobile, got, 100, -1)
	for _, m := range []string{"C", "D"} {
		if d, ok := deltas[m]; ok && d != 0 {
			t.Errorf("member %s outside the split was adjusted by %v", m, d)
		}
	}
}

// Rows written before SplitWith existed have no stored set and must keep their
// original "split across everyone" behaviour.
func TestSplitMembersForFallsBackToAllMembers(t *testing.T) {
	group := newGroup("A", "B", "C")

	for _, stored := range []string{"", "   ", ",, ,"} {
		got := splitMembersFor(group, &models.GroupExpense{SplitWith: stored})
		if len(got) != 3 {
			t.Errorf("SplitWith=%q: got %v, want all three members", stored, got)
		}
	}
}

func TestSplitMembersForTrimsWhitespace(t *testing.T) {
	group := newGroup("A", "B", "C")
	got := splitMembersFor(group, &models.GroupExpense{SplitWith: " A , B "})

	if len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Fatalf("splitMembersFor = %q, want [A B]", got)
	}
}

// Members added from the phonebook are stored as "+9198...", but the JWT
// carries a bare 10-digit number. Membership must match across both forms or
// existing users get locked out of their own groups.
func TestResolveMemberMobileAcrossFormats(t *testing.T) {
	group := newGroup("+919876543210", "9123456789")

	cases := map[string]string{
		"9876543210":      "+919876543210",
		"+919876543210":   "+919876543210",
		"+91 98765 43210": "+919876543210",
		"09876543210":     "+919876543210",
		"+919123456789":   "9123456789",
		"9123456789":      "9123456789",
	}

	for input, want := range cases {
		got, ok := resolveMemberMobile(group, input)
		if !ok {
			t.Errorf("resolveMemberMobile(%q) not found, want %q", input, want)
			continue
		}
		// Must return the *stored* string: balance updates match user_mobile
		// exactly, so returning the caller's format would update no rows.
		if got != want {
			t.Errorf("resolveMemberMobile(%q) = %q, want stored form %q", input, got, want)
		}
	}

	if _, ok := resolveMemberMobile(group, "9999999999"); ok {
		t.Error("a non-member resolved as a member")
	}
	if _, ok := resolveMemberMobile(group, ""); ok {
		t.Error("an empty number resolved as a member")
	}
}

func TestIsGroupMember(t *testing.T) {
	group := newGroup("A", "B")

	if !isGroupMember(group, "A") {
		t.Error("A should be reported as a member")
	}
	if isGroupMember(group, "Z") {
		t.Error("Z should not be reported as a member")
	}
	if isGroupMember(&models.Group{Model: gorm.Model{ID: 1}}, "A") {
		t.Error("empty group should have no members")
	}
}

func TestSettlementBalanceDeltas(t *testing.T) {
	// A pays B 100: A's debt shrinks, B's credit shrinks.
	deltas := settlementBalanceDeltas("A", "B", 100, 1)

	if got, want := deltas["A"], 100.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("payer delta = %v, want %v", got, want)
	}
	if got, want := deltas["B"], -100.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("receiver delta = %v, want %v", got, want)
	}
	if got := sumDeltas(deltas); math.Abs(got) > 1e-9 {
		t.Errorf("settlement deltas should net to zero, got %v", got)
	}
}

func TestSettlementBalanceDeltasIgnoresInvalidInput(t *testing.T) {
	if got := settlementBalanceDeltas("A", "", 100, 1); len(got) != 0 {
		t.Errorf("missing receiver should produce no deltas, got %v", got)
	}
	if got := settlementBalanceDeltas("", "B", 100, 1); len(got) != 0 {
		t.Errorf("missing payer should produce no deltas, got %v", got)
	}
	if got := settlementBalanceDeltas("A", "B", 0, 1); len(got) != 0 {
		t.Errorf("zero amount should produce no deltas, got %v", got)
	}
}

// A settlement must not be unwound with split maths, which is what the old
// single-code-path delete did.
func TestBalanceDeltasForDispatchesOnKind(t *testing.T) {
	group := newGroup("A", "B", "C")

	settlement := &models.GroupExpense{
		Kind:           models.GroupExpenseKindSettlement,
		PayerMobile:    "A",
		ReceiverMobile: "B",
		Amount:         90,
	}
	deltas, ok := balanceDeltasFor(group, settlement, 1)
	if !ok {
		t.Fatal("settlement with a receiver should be replayable")
	}
	if _, touched := deltas["C"]; touched {
		t.Error("a settlement between A and B must not touch C")
	}
	if got, want := deltas["A"], 90.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("payer delta = %v, want %v", got, want)
	}

	expense := &models.GroupExpense{
		Kind:        models.GroupExpenseKindExpense,
		PayerMobile: "A",
		Amount:      90,
	}
	deltas, ok = balanceDeltasFor(group, expense, 1)
	if !ok {
		t.Fatal("expense should be replayable")
	}
	// No stored split set, so it falls back to all three members.
	if got, want := deltas["C"], -30.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("member C delta = %v, want %v", got, want)
	}
}

// Legacy settlements have no recorded receiver; replaying them would silently
// produce wrong balances, so they must be reported as unreplayable instead.
func TestBalanceDeltasForRejectsLegacySettlement(t *testing.T) {
	group := newGroup("A", "B")
	legacy := &models.GroupExpense{
		Kind:        models.GroupExpenseKindSettlement,
		PayerMobile: "A",
		Amount:      50,
	}

	if _, ok := balanceDeltasFor(group, legacy, 1); ok {
		t.Error("a settlement without a receiver must not be replayable")
	}
}

// An empty Kind is what pre-migration rows carry; they must be treated as
// ordinary expenses rather than rejected.
func TestBalanceDeltasForTreatsBlankKindAsExpense(t *testing.T) {
	group := newGroup("A", "B")
	row := &models.GroupExpense{PayerMobile: "A", Amount: 100}

	deltas, ok := balanceDeltasFor(group, row, 1)
	if !ok {
		t.Fatal("a blank-kind row should be replayable as an expense")
	}
	if got, want := deltas["A"], 50.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("payer delta = %v, want %v", got, want)
	}
}
