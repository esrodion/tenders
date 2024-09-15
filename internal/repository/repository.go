package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"tenders/internal/config"
	"tenders/internal/models"

	postgres "tenders/internal/repository/db"
)

type Repository struct {
	db  *sql.DB
	cfg *config.PostgresConfig
}

func NewRepository(db *sql.DB, cfg *config.PostgresConfig) (*Repository, error) {
	var err error

	repo := &Repository{
		db:  db,
		cfg: cfg,
	}

	if repo.cfg == nil {
		repo.cfg, err = config.NewPostgresConfig()
		if err != nil {
			return nil, fmt.Errorf("repository.NewRepository: could not load postgres config: %w", err)
		}
	}

	if repo.db == nil {
		repo.db, err = postgres.NewPostgresDB(repo.cfg)
		if err != nil {
			return nil, fmt.Errorf("repository.NewRepository: could not open postgres db: %w", err)
		}
	}

	if repo.cfg.AutoMigrateUp == "true" {
		err = repo.MigrateUp()
		if err != nil {
			return nil, err
		}
	}

	return repo, nil
}

func (repo *Repository) MigrateUp() error {
	err := postgres.MigrateUp(repo.db, repo.cfg.MigrationsURL)
	if err != nil {
		return fmt.Errorf("repository.Repository.Migrate: %w", err)
	}
	return nil
}

func (repo *Repository) MigrateDown() error {
	err := postgres.MigrateDown(repo.db, repo.cfg.MigrationsURL)
	if err != nil {
		return fmt.Errorf("repository.Repository.Migrate: %w", err)
	}
	return nil
}

func (repo *Repository) UserByUsername(ctx context.Context, username string) (models.User, bool, error) {
	var user models.User
	query := `
	SELECT
		id,
		username,
		first_name,
		last_name,
		created_at,
		updated_at
	FROM employee
	WHERE username = $1
	LIMIT 1
	`
	row := repo.db.QueryRowContext(ctx, query, username)
	err := row.Scan(&user.Id, &user.Username, &user.FirstName, &user.LastName, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return user, false, nil
	} else if err != nil {
		return user, false, fmt.Errorf("repository.Repository.UserByUsername: %w", err)
	}

	return user, true, nil
}

func (repo *Repository) UserByUUID(ctx context.Context, UUID string) (models.User, bool, error) {
	var user models.User
	query := `
	SELECT
		id,
		username,
		first_name,
		last_name,
		created_at,
		updated_at
	FROM employee
	WHERE id = $1
	LIMIT 1
	`
	row := repo.db.QueryRowContext(ctx, query, UUID)
	err := row.Scan(&user.Id, &user.Username, &user.FirstName, &user.LastName, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return user, false, nil
	} else if err != nil {
		return user, false, fmt.Errorf("repository.Repository.UserByUsername: %w", err)
	}

	return user, true, nil
}

func (repo *Repository) UsersAreColleagues(ctx context.Context, userId1, userId2 string) (bool, error) {
	query := `
	SELECT
		organization_id,
		COUNT(DISTINCT user_id)
	FROM organization_responsible
	WHERE user_id IN ($1, $2)
	GROUP BY organization_id
	`

	rows, err := repo.db.QueryContext(ctx, query, userId1, userId2)
	if err != nil {
		return false, fmt.Errorf("repository.Repository.UsersAreColleagues: %w", err)
	}
	defer rows.Close()

	ok := false
	var org string
	var count int
	for rows.Next() {
		err = rows.Scan(&org, &count)
		if err != nil {
			return false, fmt.Errorf("repository.Repository.UsersAreColleagues: rows scan error: %w", err)
		}
		if count >= 2 {
			ok = true
		}
	}

	if rows.Err() != nil {
		return false, fmt.Errorf("repository.Repository.UsersAreColleagues: %w", rows.Err())
	}

	return ok, nil
}

func (repo *Repository) UserValid(ctx context.Context, userId, organizationId string) (bool, error) {
	// query organization_responsible for userId, orgId pair
	row := repo.db.QueryRowContext(ctx, "SELECT id FROM organization_responsible WHERE organization_id = $1 AND user_id = $2", organizationId, userId)
	var dummy string
	err := row.Scan(&dummy)

	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, sql.ErrNoRows) || ctx.Err() != nil:
		return false, nil
	}
	return false, fmt.Errorf("repository.Repository.UserValid: %w", err)
}

func (repo *Repository) UserOrganizationId(ctx context.Context, userId string) (organizationId string, err error) {
	query := `
	SELECT
		organization_id
	FROM organization_responsible
	WHERE user_id = $1
	LIMIT 1
	`

	row := repo.db.QueryRowContext(ctx, query, userId)
	err = row.Scan(&organizationId)
	return
}

func (repo *Repository) OrganizationByUUID(ctx context.Context, organizationId string) (org models.Organization, err error) {
	query := `
	SELECT
		id, name, description, type, created_at, updated_at
	FROM organization
	WHERE id = $1
	`

	row := repo.db.QueryRowContext(ctx, query, organizationId)
	err = row.Scan(&org.Id, &org.Name, &org.Description, &org.Type, &org.CreatedAt, &org.UpdatedAt)
	return
}

func (repo *Repository) Close() error {
	var migErr error
	if repo.cfg.AutoMigrateDown == "true" {
		migErr = repo.MigrateDown()
	}

	err := repo.db.Close()
	return errors.Join(migErr, err)
}

//// Service

func wrapRollbackErr(tx *sql.Tx, err error) error {
	rollerr := tx.Rollback()
	if rollerr == nil {
		return err
	}
	return fmt.Errorf("failed to rollback transaction after previous error: %w, %w", rollerr, err)
}

func sliceToSQLList[T string | models.ServiceType](t []T) string {
	parts := make([]string, 0, len(t))
	for _, v := range t {
		parts = append(parts, string(v))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

//// Test utils

func (repo *Repository) TestGetDB() *sql.DB {
	return repo.db
}
