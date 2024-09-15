package repository

import (
	"context"
	"fmt"
	"tenders/internal/models"
	"testing"
)

func TestAddTender(t *testing.T) {
	ctx := context.Background()
	repo := OpenTestRepo(t)
	defer repo.Close()

	employees := InsertTestInitData(t, repo.db)
	tenders := AddAllTenders(t, repo, employees)

	// cleanup
	for _, tender := range tenders {
		err := repo.DeleteTender(ctx, tender.Id)
		if err != nil {
			t.Errorf("Could not delete tender %s: %s", tender.Id, err)
		}
	}
}

func TestGetTenders(t *testing.T) {
	ctx := context.Background()
	repo := OpenTestRepo(t)
	defer repo.Close()

	employees := InsertTestInitData(t, repo.db)

	allTenders := AddAllTenders(t, repo, employees)
	if len(allTenders) < len(AllServiceTypes()) {
		t.Fatalf("Expeceted at least 2 tenders to be created using result of InsertTestInitData, got %d", len(allTenders))
	}

	// ensure tenders list without pagination and service type condition has all tenders
	tenders, err := repo.GetTenders(ctx, 0, 0, "", "", nil)
	if err != nil {
		t.Fatalf("Could not get tenders: %s", err)
	}

	if len(allTenders) != len(tenders) {
		t.Fatalf("Amount of added and received tenders does not match: %d - %d", len(allTenders), len(tenders))
	}

	// ensure tenders list without pagination and with service type condition by all possible service types has all tenders
	tenders, err = repo.GetTenders(ctx, 0, 0, "", "", AllServiceTypes())
	if err != nil {
		t.Fatalf("Could not get tenders: %s", err)
	}

	if len(allTenders) != len(tenders) {
		t.Fatalf("Amount of added and received tenders does not match: %d - %d", len(allTenders), len(tenders))
	}

	// ensure service type condition works correctly
	tenders, err = repo.GetTenders(ctx, 0, 0, "", "", []models.ServiceType{models.STConstruction})
	if err != nil {
		t.Fatalf("Could not get tenders: %s", err)
	}

	if len(allTenders) == len(tenders) {
		t.Fatal("Received complete tenders list, despite service type condition")
	}

	// ensure pagination works correctly
	// limit
	for _, lim := range []int{1, len(allTenders) / 2, len(allTenders)} {
		tenders, err = repo.GetTenders(ctx, lim, 0, "", "", nil)
		if err != nil {
			t.Fatalf("Could not get tenders: %s", err)
		}

		if len(tenders) != lim {
			t.Fatalf("Received wrong amount of tenders with limit set: expected %d, got %d", lim, len(tenders))
		}
	}

	// offset
	for _, off := range []int{1, len(allTenders) / 2, len(allTenders)} {
		tenders, err = repo.GetTenders(ctx, 0, off, "", "", nil)
		if err != nil {
			t.Fatalf("Could not get tenders: %s", err)
		}

		if len(tenders) != len(allTenders)-off {
			t.Fatalf("Received wrong amount of tenders with limit set: expected %d, got %d", len(allTenders)-off, len(tenders))
		}
	}

	// both
	for _, n := range []int{1, len(allTenders) / 2, len(allTenders)} {
		tenders, err = repo.GetTenders(ctx, n, n, "", "", nil)
		if err != nil {
			t.Fatalf("Could not get tenders: %s", err)
		}

		expectedLen := maxInt(len(allTenders)-n, n)
		if len(tenders) != expectedLen {
			t.Fatalf("Received wrong amount of tenders with limit set: expected %d, got %d", expectedLen, len(tenders))
		}
	}
}

func TestUpdateTender(t *testing.T) {
	ctx := context.Background()
	repo := OpenTestRepo(t)
	defer repo.Close()

	employees := InsertTestInitData(t, repo.db)
	tenders := AddAllTenders(t, repo, employees)
	if len(tenders) < 2 {
		t.Fatalf("Expeceted at least 2 tenders to be created using result of InsertTestInitData, got %d", len(tenders))
	}

	tenders[0].Name = "Updated name"
	err := repo.UpdateTender(ctx, tenders[0], true)
	if err != nil {
		t.Fatalf("Could not update tender: %s", err)
	}

	tender, err := repo.GetTenderByUUID(ctx, tenders[0].Id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tender.Name != tenders[0].Name {
		t.Errorf("Tender name have not been updated: expected '%s', got '%s'", tenders[0].Name, tender.Name)
	}
	if tender.Version != tenders[0].Version+1 {
		t.Errorf("Wrong tender version after update: expected '%d', got '%d'", tenders[0].Version+1, tender.Version)
	}

	// Ensure version entry have been added
	versions, err := repo.GetTenderVersions(ctx, tender.Id, tender.Version)
	if err != nil {
		t.Fatalf("Could not fetch tender version: %s", err)
	}
	if len(versions) == 0 {
		t.Fatalf("Version entry have not been added after tender update")
	}
	if len(versions) > 1 {
		t.Fatalf("Multiple entries with same version of specific tender found")
	}
	if versions[0].Id != tender.Id || versions[0].Name != tender.Name || versions[0].Version != tender.Version {
		t.Fatalf("Version entry is invalid: expected:\n%v\ngot:\n%v", tender, versions[0])
	}
}

//// Service

func AddAllTenders(t *testing.T, repo *Repository, employees map[string][]string) []models.Tender {
	var tenders []models.Tender
	ctx := context.Background()

	count := 0
	for _, serviceType := range AllServiceTypes() {
		for org, users := range employees {
			for _, user := range users {
				count++
				tender, err := repo.AddTender(ctx, models.Tender{
					Name:           fmt.Sprintf("Test tender %d - %s", count, serviceType),
					Description:    "",
					Status:         models.TenderCreated,
					ServiceType:    serviceType,
					Version:        1,
					OrganizationId: org,
					Author:         user,
				})

				if err != nil {
					t.Fatalf("Could not create tender: %s", err)
				}
				tenders = append(tenders, tender)
			}
		}
	}

	return tenders
}
