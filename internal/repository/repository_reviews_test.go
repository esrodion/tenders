package repository

import (
	"context"
	"tenders/internal/models"
	"testing"
)

func TestReviews(t *testing.T) {
	ctx := context.Background()
	repo := OpenTestRepo(t)
	defer repo.Close()

	// Insert organizations and employees
	employees := InsertTestInitData(t, repo.db)

	// Insert tenders
	tenders := AddAllTenders(t, repo, employees)

	// Insert bids
	bids := AddAllBids(t, ctx, repo, tenders, employees)

	// Add reviews
	total := 0
	for _, bid := range bids {
		for _, empl := range employees {
			for _, userId := range empl {
				err := repo.AddReview(ctx, models.BidReview{
					BidId:       bid.Id,
					UserId:      userId,
					Description: "Test review",
				})
				if err != nil {
					t.Fatal(err)
				}
				total++
			}
		}
	}

	reviews, err := repo.GetReviews(ctx, 0, 0, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews) != total {
		t.Errorf("Inserted %d reviews, GetReviews returned %d", total, len(reviews))
	}
}
