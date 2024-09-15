package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"tenders/internal/models"
)

func (repo *Repository) AddReview(ctx context.Context, review models.BidReview) error {
	query := `
	INSERT INTO proposal_reviews (proposal_id, user_id, text, updated_at)
	VALUES
		($1, $2, $3, CURRENT_TIMESTAMP)
	ON CONFLICT (proposal_id, user_id) DO UPDATE SET (text, updated_at) = ($3, CURRENT_TIMESTAMP)
	`
	_, err := repo.db.ExecContext(ctx, query, review.BidId, review.UserId, review.Description)
	if err != nil {
		return fmt.Errorf("repository.Repository.AddReview: %w", err)
	}
	return nil
}

func (repo *Repository) GetReviews(ctx context.Context, limit, offset int, tenderId, userId, authorId string) ([]models.BidReview, error) {
	query := `
	SELECT
		prv.proposal_id,
		prv.user_id,
		prv.text,
		prv.created_at,
		prv.updated_at
	FROM proposal_reviews AS prv
		INNER JOIN proposals ON (proposals.id = prv.proposal_id)
	$conditions$
	ORDER BY updated_at DESC
	LIMIT $1
	OFFSET $2
	`

	conditions := make([]string, 0, 3)
	params := make([]interface{}, 0, 5)

	if limit <= 0 {
		params = append(params, nil)
	} else {
		params = append(params, limit)
	}
	params = append(params, offset)

	if len(userId) > 0 {
		conditions = append(conditions, "prv.user_id = $$")
		params = append(params, userId)
	}

	if len(authorId) > 0 {
		conditions = append(conditions, "proposals.author_user_id = $$")
		params = append(params, authorId)
	}

	if len(tenderId) > 0 {
		conditions = append(conditions, "proposals.tender_id = $$")
		params = append(params, tenderId)
	}

	condStr := ""
	if len(conditions) > 0 {
		for i := 0; i < len(conditions); i++ {
			conditions[i] = strings.Replace(conditions[i], "$$", "$"+strconv.Itoa(i+3), -1)
		}
		condStr = "WHERE " + strings.Join(conditions, " AND ")
	}
	query = strings.Replace(query, "$conditions$", condStr, -1)

	rows, err := repo.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("repository.Repository.GetReviews: %w", err)
	}
	defer rows.Close()

	var reviews []models.BidReview
	var review models.BidReview
	for rows.Next() {
		err = rows.Scan(&review.BidId, &review.UserId, &review.Description, &review.CreatedAt, &review.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("repository.Repository.GetReviews: rows scan failed: %w", err)
		}
		reviews = append(reviews, review)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("repository.Repository.GetReviews: %w", err)
	}

	return reviews, nil
}
