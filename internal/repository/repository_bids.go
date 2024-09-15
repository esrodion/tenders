package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"tenders/internal/models"
)

func (repo *Repository) AddBid(ctx context.Context, bid models.Bid) (models.Bid, error) {
	query := `
	INSERT INTO proposals (version, tender_id, author_user_id, author_organization_id, status, name, description, created_at, updated_at)
	VALUES
		(1, $1, $2, $3, 'Created', $4, $5, DEFAULT, DEFAULT)
	RETURNING
		id, version, status, created_at, updated_at
	`

	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return bid, fmt.Errorf("repository.Repository.AddBid: failed to start transaction: %w", err)
	}

	var userId, orgId interface{}
	if bid.AuthorType == models.AuthorOrganization {
		orgId = bid.AuthorId
		userId = nil
	} else {
		if len(bid.OrganizationId) == 0 {
			bid.OrganizationId, err = repo.UserOrganizationId(ctx, bid.AuthorId)
			if err != nil {
				return bid, fmt.Errorf("repository.Repository.AddBid: %w", err)
			}
		}
		userId = bid.AuthorId
		orgId = bid.OrganizationId
	}

	row := tx.QueryRowContext(ctx, query, bid.TenderId, userId, orgId, bid.Name, bid.Description)
	err = row.Scan(&bid.Id, &bid.Version, &bid.Status, &bid.CreatedAt, &bid.UpdatedAt)
	if err != nil {
		return bid, fmt.Errorf("repository.Repository.AddBid: scan failed: %w", wrapRollbackErr(tx, err))
	}

	err = repo.AddBidVersion(ctx, bid, tx)
	if err != nil {
		return bid, fmt.Errorf("repository.Repository.AddBid: %w", wrapRollbackErr(tx, err))
	}

	err = tx.Commit()
	if err != nil {
		return bid, fmt.Errorf("repository.Repository.AddBid: failed to commit transaction: %w", err)
	}

	return bid, nil
}

func (repo *Repository) prepBidsQuery(limit, offset int, userId, tenderId, UUID string) (query string, queryParams []interface{}) {
	query = `
	SELECT
		id, version, tender_id, author_user_id, author_organization_id, status, name, description, created_at, updated_at
	FROM proposals
	$conditions$
	ORDER BY name
	LIMIT $1
	OFFSET $2
	`

	queryParams = make([]interface{}, 0, 5)
	conditions := make([]string, 0, 3)

	if limit <= 0 {
		queryParams = append(queryParams, nil)
	} else {
		queryParams = append(queryParams, limit)
	}
	queryParams = append(queryParams, offset)

	if len(userId) > 0 {
		queryParams = append(queryParams, userId)
		conditions = append(conditions, "author_user_id = $$")
	}
	if len(tenderId) > 0 {
		queryParams = append(queryParams, tenderId)
		conditions = append(conditions, "tender_id = $$")
	}
	if len(UUID) > 0 {
		queryParams = append(queryParams, UUID)
		conditions = append(conditions, "id = $$")
	}

	condStr := ""
	if len(conditions) > 0 {
		for i := 0; i < len(conditions); i++ {
			conditions[i] = strings.Replace(conditions[i], "$$", "$"+strconv.Itoa(i+3), -1)
		}
		condStr = "WHERE " + strings.Join(conditions, " AND ")
	}
	query = strings.Replace(query, "$conditions$", condStr, -1)

	return query, queryParams
}

func (repo *Repository) GetBids(ctx context.Context, limit, offset int, userId, tenderId string) ([]models.Bid, error) {
	query, params := repo.prepBidsQuery(limit, offset, userId, tenderId, "")

	rows, err := repo.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("repository.Repository.GetBids: %w", err)
	}
	defer rows.Close()

	var result []models.Bid
	var bid models.Bid
	var suserId, sorganizationId interface{}
	for rows.Next() {
		err = rows.Scan(&bid.Id, &bid.Version, &bid.TenderId, &suserId, &sorganizationId, &bid.Status, &bid.Name, &bid.Description, &bid.CreatedAt, &bid.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("repository.Repository.GetBids: rows scan error: %w", err)
		}
		bid.UserId = readUUID(suserId)
		bid.OrganizationId = readUUID(sorganizationId)

		if len(bid.UserId) == 0 {
			bid.AuthorType = models.AuthorOrganization
			bid.AuthorId = bid.OrganizationId
		} else {
			bid.AuthorType = models.AuthorUser
			bid.AuthorId = bid.UserId
		}
		result = append(result, bid)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("repository.Repository.GetBids: %w", err)
	}

	return result, nil
}

func (repo *Repository) GetBidByUUID(ctx context.Context, UUID string) (models.Bid, error) {
	var bid models.Bid
	var suserId, sorganizationId interface{}
	query, params := repo.prepBidsQuery(1, 0, "", "", UUID)
	row := repo.db.QueryRowContext(ctx, query, params...)
	err := row.Scan(&bid.Id, &bid.Version, &bid.TenderId, &suserId, &sorganizationId, &bid.Status, &bid.Name, &bid.Description, &bid.CreatedAt, &bid.UpdatedAt)
	if err != nil {
		return bid, fmt.Errorf("repository.Repository.GetBidByUUID: %w", err)
	}
	bid.UserId = readUUID(suserId)
	bid.OrganizationId = readUUID(sorganizationId)

	if len(bid.UserId) == 0 {
		bid.AuthorType = models.AuthorOrganization
		bid.AuthorId = bid.OrganizationId
	} else {
		bid.AuthorType = models.AuthorUser
		bid.AuthorId = bid.UserId
	}
	return bid, nil
}

func (repo *Repository) UpdateBid(ctx context.Context, bid models.Bid, incrementVersion bool) error {
	query := `
	UPDATE proposals
	SET (version, status, name, description, updated_at) = ($1, $2, $3, $4, CURRENT_TIMESTAMP)
	WHERE id = $5
	`

	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repository.Repository.UpdateBid: failed to start transaction: %w", err)
	}

	if incrementVersion {
		bid.Version++
	}
	_, err = tx.ExecContext(ctx, query, bid.Version, bid.Status, bid.Name, bid.Description, bid.Id)
	if err != nil {
		return fmt.Errorf("repository.Repository.UpdateBid: %w", wrapRollbackErr(tx, err))
	}

	if incrementVersion {
		err = repo.AddBidVersion(ctx, bid, tx)
		if err != nil {
			return fmt.Errorf("repository.Repository.UpdateBid: %w", wrapRollbackErr(tx, err))
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("repository.Repository.UpdateBid: failed to commit transaction: %w", err)
	}

	return nil
}

func (repo *Repository) DeleteBid(ctx context.Context, UUID string) error {
	_, err := repo.db.Exec("DELETE FROM proposals WHERE id = $1", UUID)
	if err != nil {
		return fmt.Errorf("repository.Repository.DeleteBid: %w", err)
	}
	return nil
}

//// Versions

func (repo *Repository) AddBidVersion(ctx context.Context, bid models.Bid, tx *sql.Tx) error {
	query := `
	INSERT INTO proposals_versions (id, version, tender_id, author_user_id, author_organization_id, status, name, description, created_at, updated_at)
	VALUES
		($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	var err error
	var userId, orgId interface{}
	if bid.AuthorType == models.AuthorOrganization {
		orgId = bid.AuthorId
		userId = nil
	} else {
		orgId = bid.OrganizationId
		userId = bid.UserId
	}

	if tx == nil {
		_, err = repo.db.ExecContext(ctx, query, bid.Id, bid.Version, bid.TenderId, userId, orgId, bid.Status, bid.Name, bid.Description, bid.CreatedAt, bid.UpdatedAt)
	} else {
		_, err = tx.ExecContext(ctx, query, bid.Id, bid.Version, bid.TenderId, userId, orgId, bid.Status, bid.Name, bid.Description, bid.CreatedAt, bid.UpdatedAt)
	}
	if err != nil {
		return fmt.Errorf("repository.Repository.AddBidVersion: scan failed: %w", err)
	}

	return nil
}

func (repo *Repository) GetBidVersions(ctx context.Context, UUID string, version int) ([]models.Bid, error) {
	query := `
	SELECT 	id, version, tender_id, author_user_id, author_organization_id, status, name, description, created_at, updated_at
	FROM proposals_versions
	WHERE id = $1 AND ($2 <= 0 OR version = $2)
	ORDER BY updated_at DESC
	`

	rows, err := repo.db.Query(query, UUID, version)
	if err != nil {
		return nil, fmt.Errorf("repository.Repository.GetBidVersions: %w", err)
	}
	defer rows.Close()

	var result []models.Bid
	var bid models.Bid
	for rows.Next() {
		err = rows.Scan(&bid.Id, &bid.Version, &bid.TenderId, &bid.UserId, &bid.OrganizationId, &bid.Status, &bid.Name, &bid.Description, &bid.CreatedAt, &bid.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("repository.Repository.GetBidVersions: rows scan error: %w", err)
		}
		result = append(result, bid)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("repository.Repository.GetBidVersions: %w", rows.Err())
	}

	return result, nil
}

//// Service

func readUUID(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}
