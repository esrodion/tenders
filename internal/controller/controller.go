package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"tenders/internal/models"
)

type Service interface {
	AddTender(ctx context.Context, username string, tender models.Tender) (models.Tender, error)
	GetTenders(ctx context.Context, limit, offset int, tenderId, userId string, serviceType []models.ServiceType) ([]models.Tender, error)
	GetUserTenders(ctx context.Context, username string, limit, offset int) ([]models.Tender, error)
	GetTenderStatus(ctx context.Context, username, tenderId string) (models.TenderStatus, error)
	SetTenderStatus(ctx context.Context, username, tenderId string, status models.TenderStatus) (models.Tender, error)
	EditTender(ctx context.Context, username, tenderId string, changes map[string]string) (models.Tender, error)
	RollbackTender(ctx context.Context, username, tenderId string, version int) (models.Tender, error)

	AddBid(ctx context.Context, bid models.Bid) (models.Bid, error)
	GetUserBids(ctx context.Context, username string, limit, offset int) ([]models.Bid, error)
	GetTenderBids(ctx context.Context, username, tenderId string, limit, offset int) ([]models.Bid, error)
	GetBidStatus(ctx context.Context, username, bidId string) (models.BidStatus, error)
	SetBidStatus(ctx context.Context, username, bidId string, status models.BidStatus) (models.Bid, error)
	EditBid(ctx context.Context, username, bidId string, changes map[string]string) (models.Bid, error)
	BidApproval(ctx context.Context, username, bidId string, status models.ApproveType) (models.Bid, error)
	BidFeedback(ctx context.Context, username, bidId, feedback string) (models.Bid, error)
	BidRollback(ctx context.Context, username, bidId string, version int) (models.Bid, error)
	PastUserBidsReviews(ctx context.Context, tenderId, requesterName, authorName string, limit, offset int) ([]models.BidReview, error)
}

type Controller struct {
	service Service
}

func NewController(service Service) *Controller {
	return &Controller{service: service}
}

//// Tenders

// GET /api/ping
func (c *Controller) Ping(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

// GET /api/tenders
func (c *Controller) GetTenders(w http.ResponseWriter, r *http.Request) {
	var serviceTypes []models.ServiceType

	query := r.URL.Query()

	limit, err := c.getQueryInt(query, "limit")
	if err != nil {
		c.errorResponse(w, http.StatusBadRequest, "invalid value of 'limit' query parameter: "+query.Get("limit"))
		return
	}

	offset, err := c.getQueryInt(query, "offset")
	if err != nil {
		c.errorResponse(w, http.StatusBadRequest, "invalid value of 'limit' query parameter: "+query.Get("limit"))
		return
	}

	for _, str := range query["service_type"] {
		t := models.ServiceType(str)
		if models.ValidServiceType(t) {
			serviceTypes = append(serviceTypes, t)
			continue
		}
		c.errorResponse(w, http.StatusBadRequest, "invalid service type supplied: "+string(t))
		return
	}

	tenders, err := c.service.GetTenders(r.Context(), limit, offset, "", "", serviceTypes)
	if err != nil {
		c.errorResponse(w, http.StatusInternalServerError, "could not fetch tenders")
		return
	}

	c.marshalResponse(w, tenders)
}

// POST /api/tenders/new
func (c *Controller) NewTender(w http.ResponseWriter, r *http.Request) {
	data, err := c.readBody(r.Body)
	if err != nil {
		c.errorResponse(w, http.StatusInternalServerError, "could not read request body")
		return
	}

	req, err := ParseNewTenderReq(data)
	if err != nil {
		c.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	tender, err := c.service.AddTender(r.Context(), req.AuthorUsername, models.Tender{
		Name:           req.Name,
		Description:    req.Description,
		ServiceType:    req.ServiceType,
		Status:         req.Status,
		OrganizationId: req.OrganizationId,
	})
	if err != nil {
		c.serviceErrorResponse(w, err)
		return
	}

	c.marshalResponse(w, tender)
}

// GET /api/tenders/my
func (c *Controller) MyTenders(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	limit, err := c.getQueryInt(query, "limit")
	if err != nil {
		c.errorResponse(w, http.StatusBadRequest, "invalid value of 'limit' query parameter: "+query.Get("limit"))
		return
	}

	offset, err := c.getQueryInt(query, "offset")
	if err != nil {
		c.errorResponse(w, http.StatusBadRequest, "invalid value of 'limit' query parameter: "+query.Get("limit"))
		return
	}

	username := query.Get("username")
	if len(username) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty username supplied")
		return
	}

	tenders, err := c.service.GetUserTenders(r.Context(), username, limit, offset)
	if err != nil {
		c.serviceErrorResponse(w, err)
		return
	}

	c.marshalResponse(w, tenders)
}

// GET /api/tenders/{tenderId}/status
func (c *Controller) TenderStatus(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	username := query.Get("username")
	if len(username) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty username supplied")
		return
	}

	tenderId := r.PathValue("tenderId")
	if len(tenderId) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty tenderId supplied")
		return
	}

	status, err := c.service.GetTenderStatus(r.Context(), username, tenderId)
	if err != nil {
		c.serviceErrorResponse(w, err)
		return
	}

	fmt.Fprint(w, status)
}

// PUT /api/tenders/{tenderId}/status
func (c *Controller) SetTenderStatus(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	username := query.Get("username")
	if len(username) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty username supplied")
		return
	}

	tenderId := r.PathValue("tenderId")
	if len(tenderId) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty tenderId supplied")
		return
	}

	status := models.TenderStatus(query.Get("status"))
	if !models.ValidTenderStatus(status) {
		c.errorResponse(w, http.StatusBadRequest, "empty or invalid status supplied")
		return
	}

	tender, err := c.service.SetTenderStatus(r.Context(), username, tenderId, status)
	if err != nil {
		c.serviceErrorResponse(w, err)
		return
	}

	c.marshalResponse(w, tender)
}

// PATCH /api/tenders/{tenderId}/edit
func (c *Controller) EditTender(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	username := query.Get("username")
	if len(username) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty username supplied")
		return
	}

	tenderId := r.PathValue("tenderId")
	if len(tenderId) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty tenderId supplied")
		return
	}

	data, err := c.readBody(r.Body)
	if err != nil {
		c.errorResponse(w, http.StatusInternalServerError, "could not read request body")
		return
	}
	req, err := ParseTenderChangeReq(data)
	if err != nil {
		c.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	tender, err := c.service.EditTender(r.Context(), username, tenderId, req)
	if err != nil {
		c.serviceErrorResponse(w, err)
		return
	}

	c.marshalResponse(w, tender)
}

// PUT /api/tenders/{tenderId}/rollback/{version}
func (c *Controller) RollbackTender(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	username := query.Get("username")
	if len(username) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty username supplied")
		return
	}

	tenderId := r.PathValue("tenderId")
	if len(tenderId) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty tenderId supplied")
		return
	}

	versionStr := r.PathValue("version")
	if len(tenderId) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty version supplied")
		return
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		c.errorResponse(w, http.StatusBadRequest, "malformed version number supplied")
		return
	}

	tender, err := c.service.RollbackTender(r.Context(), username, tenderId, version)
	if err != nil {
		c.serviceErrorResponse(w, err)
		return
	}

	c.marshalResponse(w, tender)
}

//// Bids

// POST /api/bids/new
func (c *Controller) NewBid(w http.ResponseWriter, r *http.Request) {
	data, err := c.readBody(r.Body)
	if err != nil {
		c.errorResponse(w, http.StatusInternalServerError, "could not read request body")
		return
	}

	req, err := ParseNewBidReq(data)
	if err != nil {
		c.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	bid, err := c.service.AddBid(r.Context(), models.Bid{
		Name:        req.Name,
		Description: req.Description,
		Status:      models.BidCreated,
		TenderId:    req.TenderId,
		AuthorType:  req.AuthorType,
		AuthorId:    req.AuthorId,
	})
	if err != nil {
		c.serviceErrorResponse(w, err)
		return
	}

	c.marshalResponse(w, bid)
}

// GET /api/bids/my
func (c *Controller) MyBids(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	limit, err := c.getQueryInt(query, "limit")
	if err != nil {
		c.errorResponse(w, http.StatusBadRequest, "invalid value of 'limit' query parameter: "+query.Get("limit"))
		return
	}

	offset, err := c.getQueryInt(query, "offset")
	if err != nil {
		c.errorResponse(w, http.StatusBadRequest, "invalid value of 'limit' query parameter: "+query.Get("limit"))
		return
	}

	username := query.Get("username")
	if len(username) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty username supplied")
		return
	}

	bids, err := c.service.GetUserBids(r.Context(), username, limit, offset)
	if err != nil {
		c.serviceErrorResponse(w, err)
		return
	}

	c.marshalResponse(w, bids)
}

// GET /api/bids/{tenderId}/list
func (c *Controller) TenderBids(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	limit, err := c.getQueryInt(query, "limit")
	if err != nil {
		c.errorResponse(w, http.StatusBadRequest, "invalid value of 'limit' query parameter: "+query.Get("limit"))
		return
	}

	offset, err := c.getQueryInt(query, "offset")
	if err != nil {
		c.errorResponse(w, http.StatusBadRequest, "invalid value of 'limit' query parameter: "+query.Get("limit"))
		return
	}

	tenderId := r.PathValue("tenderId")
	if len(tenderId) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty tenderId supplied")
		return
	}

	username := query.Get("username")
	if len(username) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty username supplied")
		return
	}

	bids, err := c.service.GetTenderBids(r.Context(), username, tenderId, limit, offset)
	if err != nil {
		c.serviceErrorResponse(w, err)
		return
	}

	c.marshalResponse(w, bids)
}

// GET /api/bids/{bidId}/status
func (c *Controller) BidStatus(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	username := query.Get("username")
	if len(username) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty username supplied")
		return
	}

	bidId := r.PathValue("bidId")
	if len(bidId) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty bidId supplied")
		return
	}

	status, err := c.service.GetBidStatus(r.Context(), username, bidId)
	if err != nil {
		c.serviceErrorResponse(w, err)
	}

	fmt.Fprint(w, status)
}

// PUT /api/bids/{bidId}/status
func (c *Controller) SetBidStatus(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	username := query.Get("username")
	if len(username) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty username supplied")
		return
	}

	status := query.Get("status")
	if len(username) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty status supplied")
		return
	}

	bidId := r.PathValue("bidId")
	if len(bidId) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty bidId supplied")
		return
	}

	bid, err := c.service.SetBidStatus(r.Context(), username, bidId, models.BidStatus(status))
	if err != nil {
		c.serviceErrorResponse(w, err)
	}

	c.marshalResponse(w, bid)
}

// PATCH /api/bids/{bidId}/edit
func (c *Controller) EditBid(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	username := query.Get("username")
	if len(username) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty username supplied")
		return
	}
	bidId := r.PathValue("bidId")
	if len(bidId) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty bidId supplied")
		return
	}

	data, err := c.readBody(r.Body)
	if err != nil {
		c.errorResponse(w, http.StatusInternalServerError, "could not read request body")
		return
	}

	input := map[string]string{}
	changes := map[string]string{}
	err = json.Unmarshal(data, &input)
	if err != nil {
		c.errorResponse(w, http.StatusBadRequest, "could not parse json")
		return
	}

	if str, ok := input["name"]; ok {
		if len(str) > 100 {
			c.errorResponse(w, http.StatusBadRequest, "field name exceeds max length")
			return
		}
		changes["name"] = str
	}

	if str, ok := input["description"]; ok {
		if len(str) > 100 {
			c.errorResponse(w, http.StatusBadRequest, "field description exceeds max length")
			return
		}
		changes["description"] = str
	}

	bid, err := c.service.EditBid(r.Context(), username, bidId, changes)
	if err != nil {
		c.serviceErrorResponse(w, err)
		return
	}

	c.marshalResponse(w, bid)
}

// PUT /api/bids/{bidId}/submit_decision
func (c *Controller) BidDecision(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	username := query.Get("username")
	if len(username) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty username supplied")
		return
	}
	bidId := r.PathValue("bidId")
	if len(bidId) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty bidId supplied")
		return
	}
	decision := models.ApproveType(query.Get("decision"))
	if len(decision) == 0 || !models.ValidApproveType(decision) {
		c.errorResponse(w, http.StatusBadRequest, "empty or invalid decision supplied")
		return
	}

	bid, err := c.service.BidApproval(r.Context(), username, bidId, decision)
	if err != nil {
		c.serviceErrorResponse(w, err)
		return
	}

	c.marshalResponse(w, bid)
}

// PUT /api/bids/{bidId}/feedback
func (c *Controller) BidReview(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	username := query.Get("username")
	if len(username) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty username supplied")
		return
	}
	bidId := r.PathValue("bidId")
	if len(bidId) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty bidId supplied")
		return
	}
	feedback := query.Get("bidFeedback")
	if len(feedback) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty bidFeedback supplied")
		return
	}

	bid, err := c.service.BidFeedback(r.Context(), username, bidId, feedback)
	if err != nil {
		c.serviceErrorResponse(w, err)
		return
	}

	c.marshalResponse(w, bid)
}

// PUT /api/bids/{bidId}/rollback/{version}
func (c *Controller) BidRollback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	username := query.Get("username")
	if len(username) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty username supplied")
		return
	}
	bidId := r.PathValue("bidId")
	if len(bidId) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty bidId supplied")
		return
	}
	version, err := c.getQueryInt(query, "version")
	if err != nil {
		c.errorResponse(w, http.StatusBadRequest, "invalid value of 'version' query parameter: "+query.Get("limit"))
		return
	}

	bid, err := c.service.BidRollback(r.Context(), username, bidId, version)
	if err != nil {
		c.serviceErrorResponse(w, err)
		return
	}

	c.marshalResponse(w, bid)
}

// GET /api/bids/{tenderId}/reviews
func (c *Controller) GetBidReviews(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	authorUsername := query.Get("authorUsername")
	if len(authorUsername) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty authorUsername supplied")
		return
	}
	requesterUsername := query.Get("requesterUsername")
	if len(requesterUsername) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty requesterUsername supplied")
		return
	}
	limit, err := c.getQueryInt(query, "limit")
	if err != nil {
		c.errorResponse(w, http.StatusBadRequest, "invalid value of 'limit' query parameter: "+query.Get("limit"))
		return
	}
	offset, err := c.getQueryInt(query, "offset")
	if err != nil {
		c.errorResponse(w, http.StatusBadRequest, "invalid value of 'limit' query parameter: "+query.Get("limit"))
		return
	}
	tenderId := r.PathValue("tenderId")
	if len(tenderId) == 0 {
		c.errorResponse(w, http.StatusBadRequest, "empty tenderId supplied")
		return
	}

	reviews, err := c.service.PastUserBidsReviews(r.Context(), tenderId, requesterUsername, authorUsername, limit, offset)
	if err != nil {
		c.serviceErrorResponse(w, err)
		return
	}

	c.marshalResponse(w, reviews)
}

// Service

type ErrorResponse struct {
	Reason string `json:"reason"`
}

func (c *Controller) getQueryInt(query url.Values, key string) (int, error) {
	strs, ok := query[key]
	if ok && len(strs) > 0 {
		return strconv.Atoi(strs[0])
	}
	return 0, nil
}

func (c *Controller) errorResponse(w http.ResponseWriter, status int, text string) {
	w.WriteHeader(status)

	data, err := json.Marshal(ErrorResponse{Reason: text})
	if err != nil {
		log.Printf("contorller.Controller.errorResponse: %s", err)
		return
	}

	_, err = w.Write(data)
	if err != nil {
		log.Printf("contorller.Controller.errorResponse: %s", err)
		return
	}
}

func (c *Controller) serviceErrorResponse(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, models.ErrInvalidUser):
		c.errorResponse(w, http.StatusUnauthorized, "user does not exist or have no rights for requested action")
	case errors.Is(err, models.ErrForbidden):
		c.errorResponse(w, http.StatusForbidden, "user have no permission for requested action")
	case errors.Is(err, models.ErrNoTender):
		c.errorResponse(w, http.StatusNotFound, "requested tender does not exist or unacessible")
	case errors.Is(err, models.ErrTenderFinalized):
		c.errorResponse(w, http.StatusForbidden, "requested tender is already closed, status cannot be changed")
	case errors.Is(err, models.ErrNoBid):
		c.errorResponse(w, http.StatusNotFound, "requested bid does not exist or unacessible")
	case errors.Is(err, models.ErrNoVersion):
		c.errorResponse(w, http.StatusNotFound, "requested version does not exist")
	case errors.Is(err, models.ErrBidFinalized):
		c.errorResponse(w, http.StatusForbidden, "requested bid is already approved or rejected, status cannot be changed")
	case errors.Is(err, models.ErrBidCannotBeApprovedYet):
		c.errorResponse(w, http.StatusForbidden, "requested bid does not have enough votes to be approved")
	default:
		log.Println("controller:", err)
		c.errorResponse(w, http.StatusInternalServerError, "internal server error: "+err.Error())
	}
}

func (c *Controller) marshalResponse(w http.ResponseWriter, data any) {
	d, err := json.Marshal(data)
	if err != nil {
		c.errorResponse(w, http.StatusInternalServerError, "could not marhsal response data")
		return
	}

	_, err = w.Write(d)
	if err != nil {
		c.errorResponse(w, http.StatusInternalServerError, "could not write response data")
		return
	}
}

func (c *Controller) readBody(src io.ReadCloser) ([]byte, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}
	src.Close()
	return data, nil
}
