package repository

import (
	"context"
	"tenders/internal/models"
	"testing"
)

func TestApprovals(t *testing.T) {
	ctx := context.Background()
	repo := OpenTestRepo(t)
	defer repo.Close()

	// Insert organizations and employees
	employees := InsertTestInitData(t, repo.db)

	// Insert tenders
	tenders := AddAllTenders(t, repo, employees)

	// Insert bids
	bids := AddAllBids(t, ctx, repo, tenders, employees)

	total := 0
	for _, bid := range bids {
		count := 0
		for _, empl := range employees {
			for _, userId := range empl {
				// Add approval to every bid from every possible user.
				// Rights of approval are controlled on service layer.
				err := repo.AddBidApproval(ctx, bid.Id, userId, models.ATApprove)
				if err != nil {
					t.Fatal(err)
				}
				count++
				total++
			}
		}

		m, err := repo.ApprovalCounts(ctx, bid.Id)
		if err != nil {
			t.Fatal(err)
		}
		if m[models.ATApprove] != count {
			t.Errorf("Bid '%s' was approved %d times, repo.ApprovalCounts returned %d times", bid.Id, count, m[models.ATApprove])
		}
		if m[models.ATReject] != 0 {
			t.Errorf("Bid '%s' was not rejected even once, repo.ApprovalCounts returned %d times", bid.Id, m[models.ATReject])
		}
	}

	t.Logf("Added %d bid approvals", total)
}

func TestEmployeeCount(t *testing.T) {
	ctx := context.Background()
	repo := OpenTestRepo(t)
	defer repo.Close()

	// Insert organizations and employees
	employees := InsertTestInitData(t, repo.db)

	for org, empl := range employees {
		count, err := repo.EmployeeCount(ctx, org)
		if err != nil {
			t.Fatal(err)
		}
		if count != len(empl) {
			t.Errorf("Expected amount of employess in organization '%s' to be %d, got %d", org, len(empl), count)
		}
	}
}
