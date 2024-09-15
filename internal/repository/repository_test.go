package repository

import (
	"context"
	"database/sql"
	"tenders/internal/config"
	"tenders/internal/models"
	"testing"
)

// URL of DB to perform tests on
var TestDBConn = "postgres://test:test@localhost:5432/test?sslmode=disable"

func TestNewRepository(t *testing.T) {
	repo := OpenTestRepo(t)
	repo.Close()
}

func TestUserUtils(t *testing.T) {
	repo := OpenTestRepo(t)
	defer repo.Close()

	InsertTestInitData(t, repo.db)

	rows, err := repo.db.Query("SELECT id, username FROM employee")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var id, name string
	for rows.Next() {
		err = rows.Scan(&id, &name)
		if err != nil {
			t.Fatal(err)
		}
		user, ok, err := repo.UserByUsername(context.Background(), name)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Errorf("Expeceted user '%s' to exist", name)
		}
		if user.Id != id {
			t.Errorf("Expeceted user '%s' to have id '%s', got '%s'", name, id, user.Id)
		}

		userUUID, ok, err := repo.UserByUUID(context.Background(), user.Id)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Errorf("Expeceted user by UUID '%s' to exist", user.Id)
		}
		if userUUID.Id != id {
			t.Errorf("Expeceted user '%s' to have id '%s', got '%s'", name, id, userUUID.Id)
		}

		_, err = repo.UsersAreColleagues(context.Background(), id, id)
		if err != nil {
			t.Fatal(err)
		}
	}
}

//// Service

func OpenTestRepo(t *testing.T) *Repository {
	cfg, err := config.NewPostgresConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Conn = TestDBConn

	repo, err := NewRepository(nil, cfg)
	if err != nil {
		t.Fatalf("Could not open db by URL '%s': %s", cfg.Conn, err)
	}

	err = repo.MigrateDown() // clear potential leftovers
	if err != nil {
		t.Fatal(err)
	}

	err = repo.MigrateUp()
	if err != nil {
		t.Fatal(err)
	}

	return repo
}

func InsertTestInitData(t *testing.T, db *sql.DB) map[string][]string {
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Failed to start transaction: %s", err)
	}

	// Clear potential leftovers
	_, err = tx.Exec("TRUNCATE organization_responsible, organization, employee CASCADE")
	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to truncate test tables: %s", err)
	}

	// insert organizations

	_, err = tx.Exec(`
	INSERT INTO organization 
		(name, description, type)
	VALUES
		('IE', 'Test organization 1', 'IE'),
		('LLC', 'Test organization 2', 'LLC'),
		('JSC', 'Test organization 3', 'JSC');
	`)

	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to insert default organizations: %s", err)
	}

	// insert employees

	_, err = tx.Exec(`
	INSERT INTO employee 
		(username, first_name, last_name)
	VALUES
		('Test1', 'F Test employee 1', 'IE'),
		('Test2', 'F Test employee 2', 'LLC'),
		('Test3', 'F Test employee 3', 'JSC');
	`)

	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to insert default employees: %s", err)
	}

	// fill organization_responsible
	_, err = tx.Exec(`
	INSERT INTO organization_responsible (organization_id, user_id) SELECT 
		organization.id AS organization_id,
		employee.id AS user_id 
	FROM 
		organization 
		LEFT JOIN employee
		ON organization.name = employee.last_name;
	`)

	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to insert default organization_responsible: %s", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("Could not commit transaction: %s", err)
	}

	rows, err := db.Query("SELECT organization_id, user_id FROM organization_responsible")
	if err != nil {
		t.Fatalf("Could not fetch inserted test data: %s", err)
	}
	defer rows.Close()

	employees := make(map[string][]string)
	var user, org string
	for rows.Next() {
		rows.Scan(&org, &user)
		employees[org] = append(employees[org], user)
	}
	return employees
}

func AllServiceTypes() []models.ServiceType {
	return []models.ServiceType{models.STConstruction, models.STDelivery, models.STManufacture}
}

func maxInt(vals ...int) int {
	if len(vals) == 0 {
		return 0
	}
	n := vals[0]
	for i := 1; i < len(vals); i++ {
		if n > vals[i] {
			n = vals[i]
		}
	}
	return n
}
