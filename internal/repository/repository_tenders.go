package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"tenders/internal/models"
)

func (repo *Repository) prepTendersQuery(limit, offset int, tenderId, userId string, serviceType []models.ServiceType) (query string, queryParams []interface{}) {
	query = `
	SELECT
		id,
		version,
		organization_id,
		author_id,
		status,
		service_type,
		name,
		description,
		created_at,
		updated_at
	FROM tenders
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

	if len(tenderId) > 0 {
		conditions = append(conditions, "id = $$")
		queryParams = append(queryParams, tenderId)
	}

	if len(userId) > 0 {
		conditions = append(conditions, "author_id = $$")
		queryParams = append(queryParams, userId)
	}

	if len(serviceType) > 0 {
		conditions = append(conditions, "service_type = any($$::tender_service_type[])")
		queryParams = append(queryParams, sliceToSQLList(serviceType))
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

func (repo *Repository) GetTenders(ctx context.Context, limit, offset int, tenderId, userId string, serviceType []models.ServiceType) ([]models.Tender, error) {
	query, queryParams := repo.prepTendersQuery(limit, offset, tenderId, userId, serviceType)

	rows, err := repo.db.QueryContext(ctx, query, queryParams...)
	if err != nil {
		return nil, fmt.Errorf("repository.Repository.GetTenders: %w", err)
	}
	defer rows.Close()

	var result []models.Tender
	tender := models.Tender{}
	for rows.Next() {
		err = rows.Scan(&tender.Id, &tender.Version, &tender.OrganizationId, &tender.Author, &tender.Status, &tender.ServiceType, &tender.Name, &tender.Description, &tender.CreatedAt, &tender.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("repository.Repository.GetTenders: row scan failed: %w", err)
		}
		result = append(result, tender)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("repository.Repository.GetTenders: %w", err)
	}

	return result, nil
}

func (repo *Repository) GetTenderByUUID(ctx context.Context, UUID string, tx *sql.Tx) (models.Tender, error) {
	var tender models.Tender
	query, queryParams := repo.prepTendersQuery(1, 0, UUID, "", nil)

	var rows *sql.Rows
	var err error

	if tx == nil {
		rows, err = repo.db.QueryContext(ctx, query, queryParams...)
	} else {
		rows, err = tx.QueryContext(ctx, query, queryParams...)
	}

	if err != nil {
		return tender, fmt.Errorf("repository.Repository.GetTenderByUUID: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		err = rows.Scan(&tender.Id, &tender.Version, &tender.OrganizationId, &tender.Author, &tender.Status, &tender.ServiceType, &tender.Name, &tender.Description, &tender.CreatedAt, &tender.UpdatedAt)
		if err != nil {
			return tender, fmt.Errorf("repository.Repository.GetTenderByUUID: row scan failed: %w", err)
		}
	} else {
		return tender, fmt.Errorf("repository.Repository.GetTenderByUUID: no tender found by UUID %s, %w", UUID, sql.ErrNoRows)
	}

	if rows.Err() != nil {
		return tender, fmt.Errorf("repository.Repository.GetTenderByUUID: %w", err)
	}

	return tender, nil
}

func (repo *Repository) AddTender(ctx context.Context, t models.Tender) (models.Tender, error) {
	result := t

	// Validate organization and user
	ok, err := repo.UserValid(ctx, t.Author, t.OrganizationId)
	if err != nil {
		return result, fmt.Errorf("repository.Repository.AddTender: failed to validate user: %w", err)
	}
	if !ok {
		return result, fmt.Errorf("repository.Repository.AddTender: no such user / organization pair (%s, %s)", t.Author, t.OrganizationId)
	}

	// Insert tender and version entry
	query := `
	INSERT INTO tenders 
		(version, organization_id, author_id, status, service_type, name, description) 
	VALUES 
		(1, $1, $2, $3, $4, $5, $6)
	RETURNING
		id, version, created_at
	`

	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("repository.Repository.AddTender: failed to start transaction: %w", err)
	}

	row := tx.QueryRowContext(ctx, query, t.OrganizationId, t.Author, t.Status, t.ServiceType, t.Name, t.Description)
	err = row.Scan(&result.Id, &result.Version, &result.CreatedAt)
	if err != nil {
		tx.Rollback()
		return result, fmt.Errorf("repository.Repository.AddTender: %w", err)
	}

	err = repo.AddTenderVersion(ctx, result, tx)
	if err != nil {
		tx.Rollback()
		return result, fmt.Errorf("repository.Repository.AddTender: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return result, fmt.Errorf("repository.Repository.AddTender: failed to commit transaction: %w", err)
	}

	return result, nil
}

func (repo *Repository) UpdateTender(ctx context.Context, t models.Tender, incrementVersion bool) error {
	// Validate organization and user
	ok, err := repo.UserValid(ctx, t.Author, t.OrganizationId)
	if err != nil {
		return fmt.Errorf("repository.Repository.AddTender: failed to validate user: %w", err)
	}
	if !ok {
		return fmt.Errorf("repository.Repository.AddTender: no such user / organization pair (%s, %s)", t.Author, t.OrganizationId)
	}

	// Update tender and create version entry
	query := `
	UPDATE tenders 
	SET (version, status, service_type, name, description, updated_at) =
	($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
	WHERE id = $6
	`

	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repository.Repository.UpdateTender: failed to start transaction: %w", err)
	}

	if incrementVersion {
		t.Version++
	}
	_, err = repo.db.Exec(query, t.Version, t.Status, t.ServiceType, t.Name, t.Description, t.Id)
	if err != nil {
		return fmt.Errorf("repository.Repository.UpdateTender: %w", err)
	}

	if incrementVersion {
		tender, err := repo.GetTenderByUUID(ctx, t.Id, tx)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("repository.Repository.UpdateTender: %w", err)
		}

		err = repo.AddTenderVersion(ctx, tender, tx)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("repository.Repository.UpdateTender: %w", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("repository.Repository.UpdateTender: failed to commit transaction: %w", err)
	}

	return nil
}

func (repo *Repository) DeleteTender(ctx context.Context, tenderId string) error {
	_, err := repo.db.Exec("DELETE FROM tenders WHERE id = $1", tenderId)
	if err != nil {
		return fmt.Errorf("repository.Repository.DeleteTender: %w", err)
	}
	return nil
}

//// Versions

func (repo *Repository) AddTenderVersion(ctx context.Context, t models.Tender, tx *sql.Tx) error {
	queryVersion := `
	INSERT INTO tenders_versions 
		(id, version, organization_id, author_id, status, service_type, name, description, created_at, updated_at) 
	VALUES 
		($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);
	`

	var err error
	if tx == nil {
		_, err = repo.db.ExecContext(ctx, queryVersion, t.Id, t.Version, t.OrganizationId, t.Author, t.Status, t.ServiceType, t.Name, t.Description, t.CreatedAt, t.UpdatedAt)
	} else {
		_, err = tx.ExecContext(ctx, queryVersion, t.Id, t.Version, t.OrganizationId, t.Author, t.Status, t.ServiceType, t.Name, t.Description, t.CreatedAt, t.UpdatedAt)
	}

	if err != nil {
		return fmt.Errorf("repository.Repository.AddTenderVersion: %w", err)
	}
	return nil
}

func (repo *Repository) GetTenderVersions(ctx context.Context, UUID string, version int) ([]models.Tender, error) {
	query := `
	SELECT
		id,
		version,
		organization_id,
		author_id,
		status,
		service_type,
		name,
		description,
		created_at,
		updated_at
	FROM tenders_versions
	WHERE id = $1 AND ($2 <= 0 OR version = $2)
	ORDER BY updated_at DESC
	`

	rows, err := repo.db.QueryContext(ctx, query, UUID, version)
	if err != nil {
		return nil, fmt.Errorf("repository.Repository.GetTenderVersions: %w", err)
	}
	defer rows.Close()

	var result []models.Tender
	tender := models.Tender{}
	for rows.Next() {
		err = rows.Scan(&tender.Id, &tender.Version, &tender.OrganizationId, &tender.Author, &tender.Status, &tender.ServiceType, &tender.Name, &tender.Description, &tender.CreatedAt, &tender.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("repository.Repository.GetTenderVersions: row scan failed: %w", err)
		}
		result = append(result, tender)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("repository.Repository.GetTenderVersions: %w", err)
	}

	return result, nil
}
