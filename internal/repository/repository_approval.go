package repository

import (
	"context"
	"database/sql"
	"fmt"
	"tenders/internal/models"
)

func (repo *Repository) AddBidApproval(ctx context.Context, bidId, userId string, status models.ApproveType) error {
	// check if bid is not closed yet
	// check user's permission to approve this bid

	query := `
	INSERT INTO proposal_approval 
		(proposal_id, user_id, status, updated_at)
	VALUES
		($1, $2, $3, CURRENT_TIMESTAMP)
	ON CONFLICT (proposal_id, user_id) DO UPDATE SET (status, updated_at) = ($3, CURRENT_TIMESTAMP)
	`

	_, err := repo.db.ExecContext(ctx, query, bidId, userId, status)
	if err != nil {
		return fmt.Errorf("repository.Repository.AddBidApproval: %w", err)
	}

	// if necessary, update bid status
	return nil
}

func (repo *Repository) EmployeeCount(ctx context.Context, organizationId string) (int, error) {
	query := `
	SELECT 
		COUNT(*)
	FROM organization_responsible
	WHERE organization_id = $1
	`

	row := repo.db.QueryRowContext(ctx, query, organizationId)
	var count int
	err := row.Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	} else if err != nil {
		return 0, fmt.Errorf("repository.Repository.EmployeeCount: %w", err)
	}

	return count, nil
}

func (repo *Repository) ApprovalCounts(ctx context.Context, bidId string) (map[models.ApproveType]int, error) {
	query := `
	SELECT 
		status,
		COUNT(*)
	FROM proposal_approval
	WHERE proposal_id = $1
	GROUP BY status
	`

	rows, err := repo.db.QueryContext(ctx, query, bidId)
	if err != nil {
		return nil, fmt.Errorf("repository.Repository.ApprovalCounts: %w", err)
	}
	defer rows.Close()

	result := make(map[models.ApproveType]int)
	var at models.ApproveType
	var count int

	for rows.Next() {
		err = rows.Scan(&at, &count)
		if err != nil {
			return nil, fmt.Errorf("repository.Repository.ApprovalCounts: rows scan failed: %w", err)
		}
		result[at] = count
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("repository.Repository.ApprovalCounts: %w", err)
	}

	return result, nil
}
