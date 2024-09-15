package repository

import (
	"context"
	"tenders/internal/models"
	"testing"
)

func TestBids(t *testing.T) {
	var err error
	ctx := context.Background()
	repo := OpenTestRepo(t)
	defer repo.Close()

	// Insert organizations and employees
	employees := InsertTestInitData(t, repo.db)

	// Insert tenders
	tenders := AddAllTenders(t, repo, employees)

	// Insert bids
	bids := AddAllBids(t, ctx, repo, tenders, employees)

	// Update
	bids[0].Description = "Changed description"
	repo.UpdateBid(ctx, bids[0], true)

	ubids, err := repo.GetBids(ctx, 1, 0, bids[0].AuthorId, bids[0].TenderId)
	if err != nil {
		t.Fatal(err)
	}
	if len(ubids) == 0 {
		t.Error("Could not get updated bid")
	}

	bids[0].Status = models.BidApproved
	repo.UpdateBid(ctx, bids[0], false)

	// Delete
	for _, bid := range bids {
		versions, err := repo.GetBidVersions(ctx, bid.Id, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(versions) == 0 {
			t.Errorf("No versions created for bid '%s' during test", bid.Id)
		}
		repo.DeleteBid(ctx, bid.Id)
	}

	bids, err = repo.GetBids(ctx, 0, 0, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(bids) > 0 {
		t.Errorf("Expected to have %d bid entries, got %d", 0, len(bids))
	}
}

func AddAllBids(t *testing.T, ctx context.Context, repo *Repository, tenders []models.Tender, employees map[string][]string) []models.Bid {
	var err error
	var bids []models.Bid
	var bid models.Bid
	for _, tender := range tenders {
		for org, empl := range employees {
			bid, err = repo.AddBid(ctx, models.Bid{
				TenderId:       tender.Id,
				AuthorType:     models.AuthorUser,
				AuthorId:       empl[0],
				OrganizationId: org,
				UserId:         empl[0],
				Name:           "Test",
				Description:    "Test bid",
			})
			if err != nil {
				t.Fatal(err)
			}
			bids = append(bids, bid)
		}
	}
	return bids
}
