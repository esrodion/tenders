package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"tenders/internal/config"
	"tenders/internal/models"
	"testing"
	"time"

	gofakeit "github.com/brianvoe/gofakeit/v7"
)

const EmptyUUID = "00000000-0000-0000-0000-000000000000"

func TestAppStartup(t *testing.T) {
	app := StartupApp(t)
	StopApp(app)
}

func TestPing(t *testing.T) {
	app := StartupApp(t)
	defer StopApp(app)

	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/api/ping", app.cfg.ServerAddress), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/ping should return status code 200, got %d", resp.StatusCode)
	}
}

//// Tenders

func TestTenders(t *testing.T) {
	//"GET /api/tenders"
	app := StartupApp(t)
	defer StopApp(app)

	ids := make(map[string]bool)
	for i := rand.Int() % 20; i > 0; i-- {
		ids[AddRandomTender(t, app).Id] = true
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/api/tenders", app.cfg.ServerAddress), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/tenders should return status code 200, got %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	var tenders []models.Tender
	err = json.Unmarshal(data, &tenders)
	if err != nil {
		t.Fatal(err)
	}

	if len(tenders) != len(ids) {
		t.Fatalf("Created %d tenders, received %d", len(ids), len(tenders))
	}

	for _, tender := range tenders {
		if !ids[tender.Id] {
			t.Error("Received tender via '/api/tenders', that have not been created")
		}
	}
}

func TestTendersNew(t *testing.T) {
	//"POST /api/tenders/new"
	app := StartupApp(t)
	defer StopApp(app)

	tester := func(body string, testName string, expectedStatus int) []byte {
		return ReqTest(t, app, "POST", "/api/tenders/new", body, testName, expectedStatus)
	}

	template := `
	{
	"name": "Тендер 1",
	"description": "Описание тендера",
	"serviceType": "%s",
	"organizationId": "%s",
	"creatorUsername": "%s"
	}`

	_, orgId, username := RandomEmployee(t, app)
	body := fmt.Sprintf(template, "Construction", orgId, username)
	tester(body, "correct tender", http.StatusOK)

	body = fmt.Sprintf(template, "None", orgId, username)
	tester(body, "invalid tender type", http.StatusBadRequest)

	body = fmt.Sprintf(template, "Construction", orgId, "none")
	tester(body, "invalid user", http.StatusUnauthorized)

	body = fmt.Sprintf(template, "Construction", EmptyUUID, username)
	tester(body, "invalid organization", http.StatusForbidden)

	body = fmt.Sprintf(template, "Construction", orgId, username)
	body = strings.Replace(body, "Тендер 1", strings.Repeat("0123456789", 11), -1)
	tester(body, "invalid username length", http.StatusBadRequest)

	body = fmt.Sprintf(template, "Construction", orgId, username)
	body = strings.Replace(body, "Описание тендера", strings.Repeat("0123456789", 101), -1)
	tester(body, "invalid description length", http.StatusBadRequest)
}

func TestTendersMy(t *testing.T) {
	//"GET /api/tenders/my"
	app := StartupApp(t)
	defer StopApp(app)

	// add random tenders for random users
	lists := AddRandomTenders(t, app)

	// ensure, service returns correct amount and ids for each user
	tester := func(testName string, expectedStatus int, limit, offset int, username string) []byte {
		query := fmt.Sprintf("/api/tenders/my?limit=%d&offset=%d&username=%s", limit, offset, username)
		return ReqTest(t, app, "GET", query, "", testName, expectedStatus)
	}

	var received []models.Tender
	for username, tenders := range lists {
		body := tester("check tenders list", http.StatusOK, 0, 0, username)
		err := json.Unmarshal(body, &received)
		if err != nil {
			t.Fatal(err)
		}
		if len(received) != len(tenders) {
			t.Errorf("expected to receive %d tenders, got %d", len(tenders), len(received))
		}
	}

	// ensure, service returns correct errors on invalid request
	tester("check tenders list - unathorized", http.StatusUnauthorized, 0, 0, "none")
}

func TestTenderGetStatus(t *testing.T) {
	//"GET /api/tenders/{tenderId}/status"
	app := StartupApp(t)
	defer StopApp(app)

	// add tenders
	lists := AddRandomTenders(t, app)

	// ensure, every user can check his tender's status
	tester := func(testName string, expectedStatus int, tenderId, username string) []byte {
		query := fmt.Sprintf("/api/tenders/%s/status?username=%s", tenderId, username)
		return ReqTest(t, app, "GET", query, "", testName, expectedStatus)
	}

	for username, tenders := range lists {
		for _, tender := range tenders {
			status := tester("my tender status", http.StatusOK, tender.Id, username)
			if models.TenderStatus(status) != tender.Status {
				t.Fatalf("Published tender '%s' with status '%s', got status '%s'", tender.Id, tender.Status, string(status))
			}
		}
	}

	// ensure, every employee can check status of tenders owned by his organization
	// ensure, employee can not check statuses of tenders, owned by other organization, unlsess status is "Published"
	tester2 := func(testName string, expectedStatus int, tenders []models.Tender, username string) {
		for _, tender := range tenders {
			query := fmt.Sprintf("/api/tenders/{%s}/status?username=%s", tender.Id, username)
			ReqTest(t, app, "GET", query, "", testName, expectedStatus)
		}
	}

	for username1, tenders1 := range lists {
		for username2, tenders2 := range lists {
			if username1 == username2 {
				continue
			}
			if tenders1[0].OrganizationId == tenders2[0].OrganizationId {
				tester2("my organization access", http.StatusOK, tenders2, username1)
			} else {
				tester2("foreign organization access", http.StatusForbidden, tenders2, username1)
			}
		}
	}

	// publish all tenders
	for username, tenders := range lists {
		for _, tender := range tenders {
			app.service.SetTenderStatus(context.Background(), username, tender.Id, models.TenderPublished)
		}
	}
	for username := range lists {
		for _, tenders := range lists {
			tester2("published tenders access", http.StatusOK, tenders, username)
		}
	}

	// check error status codes for unauthorized user and missing tender
	username, _ := RandomPair(lists)
	tester("unauthorized user", http.StatusUnauthorized, EmptyUUID, "none")
	tester("unauthorized user", http.StatusNotFound, EmptyUUID, username)
}

func TestTenderSetStatus(t *testing.T) {
	//"PUT /api/tenders/{tenderId}/status"
	app := StartupApp(t)
	defer StopApp(app)

	// add tenders
	lists := AddRandomTenders(t, app)

	// change tender statuses
	tester := func(testName string, expectedStatus int, tenderId, username string, status models.TenderStatus) []byte {
		query := fmt.Sprintf("/api/tenders/%s/status?username=%s&status=%s", tenderId, username, status)
		return ReqTest(t, app, "PUT", query, "", testName, expectedStatus)
	}

	for username, tenders := range lists {
		for _, tender := range tenders {
			tester("publish tender", http.StatusOK, tender.Id, username, models.TenderPublished)
			tester("close tender", http.StatusOK, tender.Id, username, models.TenderClosed)
		}
	}

	// check error statuses for unauthorized user, missing tender, foreign user
	username, tenders := RandomPair(lists)

	tester("unauthorized user", http.StatusUnauthorized, EmptyUUID, "none", models.TenderPublished)
	tester("missign tender", http.StatusNotFound, EmptyUUID, username, models.TenderPublished)
	tester("tender finalized", http.StatusForbidden, tenders[0].Id, username, models.TenderPublished)
}

func TestTenderEdit(t *testing.T) {
	//"PATCH /api/tenders/{tenderId}/edit"
	app := StartupApp(t)
	defer StopApp(app)

	// add tenders
	lists := AddRandomTenders(t, app)

	// change tenders randomly, confirm changes are settled and none wrong field is changed
	tester := func(testName string, expectedStatus int, body, tenderId, username string) []byte {
		query := fmt.Sprintf("/api/tenders/%s/edit?username=%s", tenderId, username)
		return ReqTest(t, app, "PATCH", query, body, testName, expectedStatus)
	}

	makeBody := func(name, description, stype string) string {
		m := make(map[string]string)
		if len(name) > 0 {
			m["name"] = name
		}
		if len(description) > 0 {
			m["description"] = description
		}
		if len(stype) > 0 {
			m["serviceType"] = stype
		}
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	stypes := []models.ServiceType{models.STConstruction, models.STDelivery, models.STManufacture}
	var result models.Tender
	for username, tenders := range lists {
		for _, tender := range tenders {
			newDescription, newType := gofakeit.BS(), string(stypes[rand.Int()%3])

			body := makeBody("", newDescription, newType)
			resp := tester("edit tender", http.StatusOK, body, tender.Id, username)
			err := json.Unmarshal(resp, &result)
			if err != nil {
				t.Fatal(err)
			}
			if result.Description != newDescription || result.ServiceType != models.ServiceType(newType) {
				t.Fatalf("changed description and type to %s and %s, got: %s and %s", newDescription, newType, result.Description, result.ServiceType)
			}

			tender.Description = newDescription
			tender.ServiceType = models.ServiceType(newType)
			tender.Author = "" // author and updatedAt are not returned by the service
			tender.UpdatedAt = time.Time{}
			if !structsEqual(t, tender, result) {
				t.Fatalf("exepected %v, got %v", tender, result)
			}
		}
	}

	// check error statuses: unauthorized, forbidden, missing tender
	username, tenders := RandomPair(lists)
	tender := tenders[0]

	body := makeBody("", "test change", "Construction")
	tester("edit unauthorized", http.StatusUnauthorized, body, tender.Id, "none")
	tester("edit no tender", http.StatusNotFound, body, EmptyUUID, username)

	for uname, tenders := range lists {
		if tenders[0].OrganizationId != tender.OrganizationId {
			tester("edit forbidden", http.StatusForbidden, body, tender.Id, uname)
			break
		}
	}

	// input constraints
	body = makeBody(strings.Repeat("0123456789", 11), "test change", "Construction")
	tester("name constraint", http.StatusBadRequest, body, tender.Id, username)

	body = makeBody("", strings.Repeat("0123456789", 51), "Construction")
	tester("description constraint", http.StatusBadRequest, body, tender.Id, username)

	body = makeBody("", "", "-")
	tester("service type constraint", http.StatusBadRequest, body, tender.Id, username)
}

func TestTenderRollbackTender(t *testing.T) {
	//"PUT /api/tenders/{tenderId}/rollback/{version}"
	app := StartupApp(t)
	defer StopApp(app)

	// add tenders
	lists := AddRandomTenders(t, app)

	// update tenders to push version numbrs
	for _, tenders := range lists {
		for _, tender := range tenders {
			newDescription := "updated version"
			tender.Description = newDescription
			err := app.repo.UpdateTender(context.Background(), tender, true)
			if err != nil {
				t.Fatal(err)
			}
			utender, err := app.repo.GetTenderByUUID(context.Background(), tender.Id, nil)
			if err != nil {
				t.Fatal(err)
			}
			if utender.Version != tender.Version+1 {
				t.Errorf("Expected verion of tender '%s' to be %d, got %d", tender.Id, tender.Version+1, utender.Version)
			}
		}
	}

	// rollback versions
	tester := func(testName string, expectedStatus, version int, tenderId, username string) []byte {
		query := fmt.Sprintf("/api/tenders/%s/rollback/%d?username=%s", tenderId, version, username)
		return ReqTest(t, app, "PUT", query, "", testName, expectedStatus)
	}

	var updated models.Tender
	for username, tenders := range lists {
		for _, tender := range tenders {
			resp := tester("tender rollback", http.StatusOK, tender.Version, tender.Id, username)
			err := json.Unmarshal(resp, &updated)
			if err != nil {
				t.Fatal(err)
			}
			if updated.Description != tender.Description {
				t.Fatalf("tender '%s' have not been rolled back", tender.Id)
			}
			if updated.Version != tender.Version+2 {
				t.Fatalf("tender '%s' version have not been correctly incremented: expected %d, got %d", tender.Id, tender.Version+2, updated.Version)
			}
		}
	}
}

//// Bids

func TestBidNew(t *testing.T) {
	//"POST /api/bids/new"
	app := StartupApp(t)
	defer StopApp(app)

	// add tenders
	lists := AddRandomTenders(t, app)

	// add bids
	template := `{
		"name": "%s",
		"description": "%s",
		"tenderId": "%s",
		"authorType": "%s",
		"authorId": "%s"
	}`

	tester := func(testName, body string, expectedStatus int) []byte {
		return ReqTest(t, app, "POST", "/api/bids/new", body, testName, expectedStatus)
	}

	var bid models.Bid
	var err error
	for _, tenders1 := range lists {
		for username, tenders2 := range lists {
			for _, tender := range tenders2 {
				_, err = app.service.SetTenderStatus(context.Background(), username, tender.Id, models.TenderPublished)
				if err != nil {
					t.Fatal(err)
				}

				name, descr := gofakeit.BuzzWord(), gofakeit.Blurb()
				body := fmt.Sprintf(template, name, descr, tender.Id, models.AuthorUser, tenders1[0].Author)
				resp := tester("add bid", body, http.StatusOK)
				err = json.Unmarshal(resp, &bid)
				if err != nil {
					t.Fatal(err)
				}
				if bid.Name != name || bid.Description != descr || bid.TenderId != tender.Id || bid.AuthorType != models.AuthorUser || bid.AuthorId != tenders1[0].Author {
					t.Fatalf("After addition of bid with name = %s, description = %s, tender id = %s, author type = %s, author = %s, got %v",
						name, descr, tender.Id, models.AuthorUser, tenders1[0].Author, bid)
				}
			}
		}
	}

	// test error statuses
	_, tenders := RandomPair(lists)
	tender := tenders[0]

	// unauthorized
	body := fmt.Sprintf(template, "name", "description", tender.Id, models.AuthorUser, EmptyUUID)
	tester("add bid empty user", body, http.StatusUnauthorized)

	body = fmt.Sprintf(template, "name", "description", tender.Id, models.AuthorOrganization, EmptyUUID)
	tester("add bid empty organozation", body, http.StatusUnauthorized)

	// no tender
	body = fmt.Sprintf(template, "name", "description", EmptyUUID, models.AuthorOrganization, tender.Author)
	tester("add bid empty organozation", body, http.StatusUnauthorized)
}

func TestBidsMy(t *testing.T) {
	//"GET /api/bids/my"
	app := StartupApp(t)
	defer StopApp(app)

	// add tenders
	AddRandomTenders(t, app)

	// add bids
	lists := AddRandomBids(t, app)

	// ensure service returns correct list for each user
	tester := func(testName string, expectedStatus, limit, offset int, username string) []byte {
		query := fmt.Sprintf("/api/bids/my?limit=%d&offset=%d&username=%s", limit, offset, username)
		return ReqTest(t, app, "GET", query, "", testName, expectedStatus)
	}

	var rbids []models.Bid
	for username, bids := range lists {
		resp := tester("get bids", http.StatusOK, 0, 0, username)
		err := json.Unmarshal(resp, &rbids)
		if err != nil {
			t.Fatal(err)
		}
		if len(rbids) != len(bids) {
			t.Fatalf("expected %d bids, got %d", len(bids), len(rbids))
		}
	}
}

func TestTenderBids(t *testing.T) {
	//"GET /api/bids/{tenderId}/list"
	app := StartupApp(t)
	defer StopApp(app)

	// add and publish all tenders
	var dummyUsername string
	userTenders := AddRandomTenders(t, app)
	for username, tenders := range userTenders {
		dummyUsername = username
		for _, tender := range tenders {
			app.service.SetTenderStatus(context.Background(), username, tender.Id, models.TenderPublished)
		}
	}

	// add bids
	ctx := context.Background()
	lists := AddRandomBids(t, app)
	tenderBids := map[string][]models.Bid{}
	for username, bids := range lists {
		for _, bid := range bids {
			tenderBids[bid.TenderId] = append(tenderBids[bid.TenderId], bid)
			_, err := app.service.SetBidStatus(ctx, username, bid.Id, models.BidPublished)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	tester := func(testName string, expectedStatus, limit, offset int, tenderId, username string) []byte {
		query := fmt.Sprintf("/api/bids/%s/list?limit=%d&offset=%d&username=%s", tenderId, limit, offset, username)
		return ReqTest(t, app, "GET", query, "", testName, expectedStatus)
	}

	var rbids []models.Bid
	for tenderId, bids := range tenderBids {
		resp := tester("get bids", http.StatusOK, 0, 0, tenderId, dummyUsername)
		err := json.Unmarshal(resp, &rbids)
		if err != nil {
			t.Fatal(err)
		}
		if len(rbids) != len(bids) {
			repobids, err := app.repo.GetBids(ctx, 0, 0, "", tenderId)
			if err != nil {
				t.Error(err)
			}
			if len(repobids) != len(bids) {
				t.Errorf("created %d bids, repository returns %d", len(bids), len(repobids))
			}
			t.Fatalf("expected %d bids, got %d", len(bids), len(rbids))
		}
	}
}

func TestBidGetStatus(t *testing.T) {
	//"GET /api/bids/{bidId}/status"
	app := StartupApp(t)
	defer StopApp(app)

	// add tenders
	userTenders := AddRandomTenders(t, app)
	for username, tenders := range userTenders {
		for _, tender := range tenders {
			app.service.SetTenderStatus(context.Background(), username, tender.Id, models.TenderPublished)
		}
	}

	// add bids
	tester := func(testName string, expectedStatus int, bidId, username string) []byte {
		query := fmt.Sprintf("/api/bids/%s/status?username=%s", bidId, username)
		return ReqTest(t, app, "GET", query, "", testName, expectedStatus)
	}

	lists := AddRandomBids(t, app)
	for username, bids := range lists {
		for _, bid := range bids {
			resp := tester("bid status", http.StatusOK, bid.Id, username)
			if models.BidStatus(resp) != bid.Status {
				t.Errorf("expected %s, got %s", bid.Status, models.BidStatus(resp))
			}
		}
	}
}

func TestBidSetStatus(t *testing.T) {
	//"PUT /api/bids/{bidId}/status"
	app := StartupApp(t)
	defer StopApp(app)

	// add tenders
	userTenders := AddRandomTenders(t, app)
	for username, tenders := range userTenders {
		for _, tender := range tenders {
			app.service.SetTenderStatus(context.Background(), username, tender.Id, models.TenderPublished)
		}
	}

	// add bids
	tester := func(testName string, expectedStatus int, bidId, username string, status models.BidStatus) []byte {
		query := fmt.Sprintf("/api/bids/%s/status?username=%s&status=%s", bidId, username, status)
		return ReqTest(t, app, "PUT", query, "", testName, expectedStatus)
	}

	lists := AddRandomBids(t, app)
	for username, bids := range lists {
		for _, bid := range bids {
			resp := tester("set bid status", http.StatusOK, bid.Id, username, models.BidPublished)
			err := json.Unmarshal(resp, &bid)
			if err != nil {
				t.Fatal(err)
			}
			if bid.Status != models.BidPublished {
				t.Errorf("expected %s, got %s", models.BidPublished, bid.Status)
			}
		}
	}
}

func TestBidEditRollback(t *testing.T) {
	//"PATCH /api/bids/{bidId}/edit"
	app := StartupApp(t)
	defer StopApp(app)

	// add tenders
	userTenders := AddRandomTenders(t, app)
	for username, tenders := range userTenders {
		for _, tender := range tenders {
			app.service.SetTenderStatus(context.Background(), username, tender.Id, models.TenderPublished)
		}
	}

	// add bids
	tester := func(testName string, expectedStatus int, body, bidId, username string) []byte {
		query := fmt.Sprintf("/api/bids/%s/edit?username=%s", bidId, username)
		return ReqTest(t, app, "PATCH", query, body, testName, expectedStatus)
	}

	body := `{"name": "test", "description": "change"}`

	lists := AddRandomBids(t, app)
	for username, bids := range lists {
		for _, bid := range bids {
			resp := tester("set bid status", http.StatusOK, body, bid.Id, username)
			err := json.Unmarshal(resp, &bid)
			if err != nil {
				t.Fatal(err)
			}
			if bid.Name != "test" || bid.Description != "change" {
				t.Errorf("no changes happened after /api/bids/{bidId}/edit call")
			}
		}
	}

	//"PUT /api/bids/{bidId}/rollback/{version}"
	tester2 := func(testName string, expectedStatus, version int, bidId, username string) []byte {
		query := fmt.Sprintf("/api/bids/%s/rollback/%d?username=%s", bidId, version, username)
		return ReqTest(t, app, "PUT", query, "", testName, expectedStatus)
	}

	for username, bids := range lists {
		for _, bid := range bids {
			oldVersion := bid.Version
			resp := tester2("set bid status", http.StatusOK, oldVersion, bid.Id, username)
			err := json.Unmarshal(resp, &bid)
			if err != nil {
				t.Fatal(err)
			}
			if bid.Name == "test" || bid.Description == "change" || bid.Version != oldVersion+2 {
				t.Errorf("version was not rolled back after /api/bids/{bidId}/rollback/{version} call")
			}
		}
	}
}

func TestBidDecision(t *testing.T) {
	var err error

	//"PUT /api/bids/{bidId}/submit_decision"
	app := StartupApp(t)
	defer StopApp(app)

	ctx := context.Background()

	// add tenders
	userTenders := AddRandomTenders(t, app)
	for username, tenders := range userTenders {
		for _, tender := range tenders {
			app.service.SetTenderStatus(context.Background(), username, tender.Id, models.TenderPublished)
		}
	}

	// add bids
	lists := AddRandomBids(t, app)

	tester := func(testName string, expectedStatus int, decision, bidId, username string) models.Bid {
		query := fmt.Sprintf("/api/bids/%s/submit_decision?username=%s&decision=%s", bidId, username, decision)
		resp := ReqTest(t, app, "PUT", query, "", testName, expectedStatus)
		bid := models.Bid{}
		if expectedStatus == http.StatusOK {
			err := json.Unmarshal(resp, &bid)
			if err != nil {
				t.Fatal(err, string(resp))
			}
		}
		return bid
	}

	bidStatusCheck := func(bidId string, expectedStatus models.BidStatus) {
		bid, err := app.repo.GetBidByUUID(ctx, bidId)
		if err != nil {
			t.Fatal(err)
		}
		if bid.Status != expectedStatus {
			t.Fatalf("expected status '%s', got '%s'", models.BidRejected, bid.Status)
		}
	}

	// ensure, decision cannot be made on closed / created bid
	_, bids := RandomPair(lists)
	_, responsibleUser := RandomOrgEmployee(t, app, TenderOrganization(t, app, bids[0].TenderId))
	tester("approve closed bid", http.StatusForbidden, string(models.ATApprove), bids[0].Id, responsibleUser)

	// publish all bids
	for username, bids := range lists {
		for i, bid := range bids {
			bids[i], err = app.service.SetBidStatus(ctx, username, bid.Id, models.BidPublished)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	// choose bid and reject it
	_, bids = RandomPair(lists)
	bid := bids[rand.Int()%len(bids)]
	_, responsibleUser = RandomOrgEmployee(t, app, TenderOrganization(t, app, bid.TenderId))
	bids[0] = tester("reject bid", http.StatusOK, string(models.ATReject), bid.Id, responsibleUser)

	// ensure, bid's status changes to rejected, when reject received
	bidStatusCheck(bid.Id, models.BidRejected)

	// chose bid and approve it
	_, bids = RandomPair(lists)
	bid = bids[rand.Int()%len(bids)]
	orgId := TenderOrganization(t, app, bid.TenderId)
	_, responsibleUser = RandomOrgEmployee(t, app, orgId)
	tester("approve bid", http.StatusOK, string(models.ATApprove), bid.Id, responsibleUser)

	count, err := app.repo.EmployeeCount(ctx, orgId)
	if err != nil {
		t.Fatal(err)
	}

	// ensure, bid's status changes to approved, when enough approves received
	if count == 1 {
		bidStatusCheck(bid.Id, models.BidApproved)
	} else {
		bidStatusCheck(bid.Id, bid.Status)
		for i := 0; i < count; i++ {
			tester("approve bid", http.StatusOK, string(models.ATApprove), bid.Id, responsibleUser)
		}
		bidStatusCheck(bid.Id, bid.Status)
		for i := 0; i < count; i++ {
			_, user := OrgEmployee(t, app, TenderOrganization(t, app, bid.TenderId), i)
			tester("approve bid", http.StatusOK, string(models.ATApprove), bid.Id, user)
		}
		bidStatusCheck(bid.Id, models.BidApproved)
	}

	// ensure, tender is closed, when any bid is approved
	tender, err := app.repo.GetTenderByUUID(ctx, bid.TenderId, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tender.Status != models.TenderClosed {
		t.Fatalf("expected status '%s', got '%s'", models.TenderClosed, tender.Status)
	}
}

func TestBidFeedback(t *testing.T) {
	//"PUT /api/bids/{bidId}/feedback"
	app := StartupApp(t)
	defer StopApp(app)

	// add tenders
	ctx := context.Background()
	userTenders := AddRandomTenders(t, app)
	for username, tenders := range userTenders {
		for _, tender := range tenders {
			app.service.SetTenderStatus(ctx, username, tender.Id, models.TenderPublished)
		}
	}

	// add bids
	tester := func(testName string, bidId, username, feedback string) []byte {
		query := fmt.Sprintf(`/api/bids/%s/feedback?username=%s&bidFeedback=%s`, bidId, username, feedback)
		return ReqTest(t, app, "PUT", query, "", testName, 0)
	}

	lists := AddRandomBids(t, app)
	for username, bids := range lists {
		for _, bid := range bids {
			_, err := app.service.SetBidStatus(ctx, username, bid.Id, models.BidPublished)
			if err != nil {
				t.Fatal(err)
			}
			resp := tester("add review", bid.Id, username, "testfeedback")
			err = json.Unmarshal(resp, &bid)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	//"GET /api/bids/{tenderId}/reviews"
	tester2 := func(testName string, limit, offset int, tenderId, authorUsername, requesterUsername string) []byte {
		query := fmt.Sprintf("/api/bids/%s/reviews?limit=%d&offset=%d&authorUsername=%s&requesterUsername=%s", tenderId, limit, offset, authorUsername, requesterUsername)
		return ReqTest(t, app, "GET", query, "", testName, 0)
	}

	count := 0
	for username, bids := range lists {
		for username2 := range lists {
			count++
			tester2("get reviews "+strconv.Itoa(count), 0, 0, bids[0].TenderId, username2, username)
		}
		if count >= 200 {
			break
		}
	}
}

//// Service

func StartupApp(t *testing.T) *App {
	gofakeit.Seed(0)

	cfg, err := config.NewConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.AutoMigrateUp = "false"
	cfg.AutoMigrateDown = "true"
	cfg.Conn = "postgres://test:test@localhost:5432/test?sslmode=disable"

	app, err := NewApp(WithConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}

	app.repo.MigrateDown() // clear potential leftovers
	app.repo.MigrateUp()

	go app.Run()
	time.Sleep(time.Second)

	InsertTestOrganizations(t, app)
	InsertTestEmployees(t, app)
	ShuffleEmployees(t, app)
	return app
}

func StopApp(app *App) {
	app.stopSig <- os.Interrupt
	<-app.Done
}

func InsertTestOrganizations(t *testing.T, app *App) {
	var tableSize int
	row := app.repo.TestGetDB().QueryRow("SELECT COUNT(*) FROM organization")
	row.Scan(&tableSize)
	if tableSize > 0 {
		return
	}

	types := []string{"IE", "LLC", "JSC"}
	count := rand.Int()%20 + 3
	lines := make([]string, 0, count)
	for i := count; i > 0; i-- {
		lines = append(lines, fmt.Sprintf("($$%s$$, $$%s$$, $$%s$$)", gofakeit.Company(), gofakeit.Blurb(), types[rand.Int()%3]))
	}

	_, err := app.repo.TestGetDB().Exec(`
	INSERT INTO organization 
		(name, description, type)
	VALUES
	` + strings.Join(lines, ",") + ";")

	if err != nil {
		t.Fatal(err)
	}
}

func InsertTestEmployees(t *testing.T, app *App) {
	var tableSize int
	row := app.repo.TestGetDB().QueryRow("SELECT COUNT(*) FROM employee")
	row.Scan(&tableSize)
	if tableSize > 0 {
		return
	}

	count := rand.Int()%100 + 20
	lines := make([]string, 0, count)
	for i := count; i > 0; i-- {
		lines = append(lines, fmt.Sprintf("($$%sn%s$$, $$%s$$, $$%s$$)", gofakeit.Username(), strconv.Itoa(i), gofakeit.FirstName(), gofakeit.LastName()))
	}

	_, err := app.repo.TestGetDB().Exec(`
	INSERT INTO employee 
		(username, first_name, last_name)
	VALUES
	` + strings.Join(lines, ",") + ";")

	if err != nil {
		t.Fatal(err)
	}
}

func ShuffleEmployees(t *testing.T, app *App) {
	var tableSize int
	row := app.repo.TestGetDB().QueryRow("SELECT COUNT(*) FROM organization_responsible")
	row.Scan(&tableSize)
	if tableSize > 0 {
		return
	}

	var id string

	rorgs, err := app.repo.TestGetDB().Query("SELECT id FROM organization")
	if err != nil {
		t.Fatal(err)
	}
	defer rorgs.Close()

	orgs := []string{}
	for rorgs.Next() {
		rorgs.Scan(&id)
		orgs = append(orgs, id)
	}

	rempls, err := app.repo.TestGetDB().Query("SELECT id FROM employee")
	if err != nil {
		t.Fatal(err)
	}
	defer rempls.Close()

	empls := []string{}
	for rempls.Next() {
		rempls.Scan(&id)
		empls = append(empls, id)
	}

	lines := make([]string, 0, len(empls))
	for _, id := range orgs {
		lines = append(lines, fmt.Sprintf("('%s', '%s')", id, empls[0]))
		empls = empls[1:]
	}

	for len(empls) > 0 {
		lines = append(lines, fmt.Sprintf("('%s', '%s')", orgs[rand.Int()%len(orgs)], empls[0]))
		empls = empls[1:]
	}

	_, err = app.repo.TestGetDB().Exec(`
	INSERT INTO organization_responsible (organization_id, user_id)
	VALUES
	` + strings.Join(lines, ",") + ";")

	if err != nil {
		t.Fatal(err)
	}
}

func RandomEmployee(t *testing.T, app *App) (userId string, orgId string, username string) {
	row := app.repo.TestGetDB().QueryRow(`
	SELECT COUNT(*)
	FROM organization_responsible`)

	var count int
	err := row.Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	row = app.repo.TestGetDB().QueryRow(`
	SELECT oresp.organization_id, oresp.user_id, empl.username
	FROM organization_responsible AS oresp
		INNER JOIN employee AS empl ON (oresp.user_id = empl.id)
	LIMIT $1
	OFFSET $2
	`, 1, rand.Int()%count)

	err = row.Scan(&orgId, &userId, &username)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func OrgEmployee(t *testing.T, app *App, organizationId string, offset int) (userId string, username string) {
	row := app.repo.TestGetDB().QueryRow(`
	SELECT oresp.user_id, empl.username
	FROM organization_responsible AS oresp
		INNER JOIN employee AS empl ON (oresp.user_id = empl.id)
	WHERE organization_id = $3	
	LIMIT $1
	OFFSET $2
	`, 1, offset, organizationId)

	err := row.Scan(&userId, &username)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func RandomOrgEmployee(t *testing.T, app *App, organizationId string) (string, string) {
	row := app.repo.TestGetDB().QueryRow(`
	SELECT COUNT(*)
	FROM organization_responsible
	WHERE organization_id = $1`, organizationId)

	var count int
	err := row.Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	return OrgEmployee(t, app, organizationId, rand.Int()%count)
}

func TenderOrganization(t *testing.T, app *App, tenderId string) (id string) {
	row := app.repo.TestGetDB().QueryRow(`
	SELECT organization_id FROM tenders WHERE id = $1
	`, tenderId)
	err := row.Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return
}

var TotalTenders int = 0

func AddRandomTender(t *testing.T, app *App) models.Tender {
	userId, orgId, username := RandomEmployee(t, app)
	tender, err := app.service.AddTender(context.Background(), username, models.Tender{
		OrganizationId: orgId,
		Author:         userId,
		Status:         "Created",
		ServiceType:    models.STConstruction,
		Name:           fmt.Sprintf("%s_%d", gofakeit.BuzzWord(), TotalTenders),
		Description:    username,
	})
	if err != nil {
		t.Fatal(err)
	}
	TotalTenders++
	return tender
}

func AddRandomTenders(t *testing.T, app *App) map[string][]models.Tender {
	lists := make(map[string][]models.Tender)
	for i := rand.Int()%20 + 10; i > 0; i-- {
		for j := rand.Int()%4 + 1; j > 0; j-- {
			tender := AddRandomTender(t, app)
			// description contains author's username in test environment
			lists[tender.Description] = append(lists[tender.Description], tender)
		}
	}
	return lists
}

func AddRandomBids(t *testing.T, app *App) map[string][]models.Bid {
	result := map[string][]models.Bid{}
	query := `
	SELECT
		tenders.id,
		authors.user_id,
		authors.organization_id,
		employee.username AS username
	FROM
		(SELECT id AS id, ROW_NUMBER() OVER(ORDER BY id) AS rnumber FROM tenders) AS tenders
		JOIN (SELECT COUNT(*) AS n FROM tenders) AS count 
			ON (true)
		JOIN (SELECT 
				organization_responsible.user_id, 
				organization_responsible.organization_id, 
				floor(random() * count.n)::int + 1 AS pos 
			FROM 
				organization_responsible
				JOIN (SELECT COUNT(*) AS n FROM tenders) AS count ON (true)) AS authors 
			ON (tenders.rnumber = authors.pos)
		JOIN employee ON (authors.user_id = employee.id)
	`

	rows, err := app.repo.TestGetDB().Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var bid models.Bid
	var username string
	ctx := context.Background()
	//aTypes := []models.AuthorType{models.AuthorOrganization, models.AuthorUser}
	for rows.Next() {
		err = rows.Scan(&bid.TenderId, &bid.UserId, &bid.OrganizationId, &username)
		if err != nil {
			t.Fatal(err)
		}
		bid.Name = gofakeit.BS()
		bid.Description = gofakeit.Blurb()
		bid.AuthorType = models.AuthorUser //aTypes[rand.Int()%2]
		if bid.AuthorType == models.AuthorOrganization {
			bid.AuthorId = bid.OrganizationId
		} else {
			bid.AuthorId = bid.UserId
		}
		bid, err = app.repo.AddBid(ctx, bid)
		if err != nil {
			t.Fatal(err)
		}
		result[username] = append(result[username], bid)
	}

	return result
}

func ReqTest(t *testing.T, app *App, method, endpoint, body, testName string, expectedStatus int) []byte {
	var reader io.Reader
	if len(body) > 0 {
		reader = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, fmt.Sprintf("http://%s%s", app.cfg.ServerAddress, endpoint), reader)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if expectedStatus == 0 && resp.StatusCode != 400 && resp.StatusCode < 500 {
		return respBody
	}
	if resp.StatusCode != expectedStatus {
		t.Fatalf("%s %s '%s' test should return status code %d, got %d, body:\n%s", method, endpoint, testName, expectedStatus, resp.StatusCode, string(respBody))
	}
	return respBody
}

func RandomPair[S comparable, T any](m map[S]T) (S, T) {
	var a S
	var b T
	if len(m) == 0 {
		panic("empty collection supplied")
	}
	limit := rand.Int() % len(m)
	count := 0
	for a, b = range m {
		if count >= limit {
			break
		}
		count++
	}
	return a, b
}

// compares fields of simple types
func structsEqual(t *testing.T, obj1, obj2 interface{}) bool {
	m1, m2 := map[string]interface{}{}, map[string]interface{}{}

	data, err := json.Marshal(obj1)
	if err != nil {
		t.Fatal(err)
	}
	err = json.Unmarshal(data, &m1)
	if err != nil {
		t.Fatal(err)
	}

	data, err = json.Marshal(obj2)
	if err != nil {
		t.Fatal(err)
	}
	err = json.Unmarshal(data, &m2)
	if err != nil {
		t.Fatal(err)
	}

	for k, v := range m1 {
		if v != m2[k] {
			return false
		}
	}

	for k, v := range m2 {
		if v != m1[k] {
			return false
		}
	}

	return true
}
