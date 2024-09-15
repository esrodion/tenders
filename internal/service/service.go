package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"tenders/internal/models"
	"tenders/internal/repository"
)

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

//// Tenders

func (s *Service) GetTenders(ctx context.Context, limit, offset int, tenderId, userId string, serviceType []models.ServiceType) ([]models.Tender, error) {
	tenders, err := s.repo.GetTenders(ctx, limit, offset, tenderId, userId, serviceType)
	if err != nil {
		return nil, fmt.Errorf("service.Service.GetTenders: %w", err)
	}
	return tenders, nil
}

func (s *Service) AddTender(ctx context.Context, username string, tender models.Tender) (models.Tender, error) {
	user, err := s.userByUsername(ctx, username)
	if err != nil {
		return tender, fmt.Errorf("service.Service.AddTender: %w", err)
	}

	valid, err := s.repo.UserValid(ctx, user.Id, tender.OrganizationId)
	if err != nil {
		return tender, fmt.Errorf("service.Service.AddTender: %w", err)
	}
	if !valid {
		return tender, fmt.Errorf("service.Service.AddTender: %w: %s", models.ErrForbidden, username)
	}

	tender.Author = user.Id
	tender, err = s.repo.AddTender(ctx, tender)
	if err != nil {
		return tender, fmt.Errorf("service.Service.AddTender: %w", err)
	}

	return tender, nil
}

func (s *Service) GetUserTenders(ctx context.Context, username string, limit, offset int) ([]models.Tender, error) {
	user, err := s.userByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("service.Service.GetUserTenders: %w", err)
	}

	tenders, err := s.repo.GetTenders(ctx, limit, offset, "", user.Id, nil)
	if err != nil {
		return nil, fmt.Errorf("service.Service.GetUserTenders: %w", err)
	}

	return tenders, nil
}

func (s *Service) GetTenderStatus(ctx context.Context, username, tenderId string) (models.TenderStatus, error) {
	// check if username exists
	user, err := s.userByUsername(ctx, username)
	if err != nil {
		return "", fmt.Errorf("service.Service.GetTenderStatus: %w", err)
	}

	// get tender
	tender, err := s.repo.GetTenderByUUID(ctx, tenderId, nil)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("service.Service.GetTenderStatus: %w", models.ErrNoTender)
	} else if err != nil {
		return "", fmt.Errorf("service.Service.GetTenderStatus: %w", err)
	}

	// check whether user is employee of organization or not
	valid, err := s.repo.UserValid(ctx, user.Id, tender.OrganizationId)
	if err != nil {
		return "", fmt.Errorf("service.Service.GetTenderStatus: %w", err)
	}

	// if user is employee of organization owning tender, or tender is public, return status
	if valid || tender.Status == models.TenderPublished {
		return tender.Status, nil
	}

	return "", models.ErrForbidden
}

func (s *Service) SetTenderStatus(ctx context.Context, username, tenderId string, status models.TenderStatus) (models.Tender, error) {
	// check if username exists
	user, err := s.userByUsername(ctx, username)
	if err != nil {
		return models.Tender{}, fmt.Errorf("service.Service.SetTenderStatus: %w", err)
	}

	// get tender
	tender, err := s.repo.GetTenderByUUID(ctx, tenderId, nil)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Tender{}, fmt.Errorf("service.Service.SetTenderStatus: %w", models.ErrNoTender)
	} else if err != nil {
		return models.Tender{}, fmt.Errorf("service.Service.SetTenderStatus: %w", err)
	}

	// check whether user is employee of organization or not
	valid, err := s.repo.UserValid(ctx, user.Id, tender.OrganizationId)
	if err != nil {
		return models.Tender{}, fmt.Errorf("service.Service.SetTenderStatus: %w", err)
	}

	// check tender status
	if tender.Status == models.TenderClosed && status != models.TenderClosed {
		return models.Tender{}, fmt.Errorf("service.Service.SetTenderStatus: %w", models.ErrTenderFinalized)
	}

	// if user is employee of organization owning tender, change status
	if valid {
		tender.Status = status
		s.repo.UpdateTender(ctx, tender, status != models.TenderClosed)
		return tender, nil
	}

	return models.Tender{}, models.ErrForbidden
}

func (s *Service) EditTender(ctx context.Context, username, tenderId string, changes map[string]string) (models.Tender, error) {
	// check if username exists
	user, err := s.userByUsername(ctx, username)
	if err != nil {
		return models.Tender{}, fmt.Errorf("service.Service.EditTender: %w", err)
	}

	// get tender
	tender, err := s.repo.GetTenderByUUID(ctx, tenderId, nil)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Tender{}, fmt.Errorf("service.Service.EditTender: %w", models.ErrNoTender)
	} else if err != nil {
		return models.Tender{}, fmt.Errorf("service.Service.EditTender: %w", err)
	}

	// check whether user is employee of organization or not
	valid, err := s.repo.UserValid(ctx, user.Id, tender.OrganizationId)
	if err != nil {
		return models.Tender{}, fmt.Errorf("service.Service.EditTender: %w", err)
	}

	// if user is not employee of organization owning tender, return error
	if !valid {
		return models.Tender{}, models.ErrForbidden
	}

	// check tender status
	if tender.Status == models.TenderClosed {
		return models.Tender{}, fmt.Errorf("service.Service.EditTender: %w", models.ErrTenderFinalized)
	}

	// remarshal changes into tender
	data, err := json.Marshal(changes)
	if err != nil {
		return models.Tender{}, fmt.Errorf("service.Service.EditTender: %w", err)
	}
	err = json.Unmarshal(data, &tender)
	if err != nil {
		return models.Tender{}, fmt.Errorf("service.Service.EditTender: %w", err)
	}

	// update tender
	err = s.repo.UpdateTender(ctx, tender, true)
	if err != nil {
		return models.Tender{}, fmt.Errorf("service.Service.EditTender: %w", err)
	}

	return tender, nil
}

func (s *Service) RollbackTender(ctx context.Context, username, tenderId string, version int) (models.Tender, error) {
	// check if username exists
	user, err := s.userByUsername(ctx, username)
	if err != nil {
		return models.Tender{}, fmt.Errorf("service.Service.RollbackTender: %w", err)
	}

	// get tender
	tender, err := s.repo.GetTenderByUUID(ctx, tenderId, nil)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Tender{}, fmt.Errorf("service.Service.RollbackTender: %w", models.ErrNoTender)
	} else if err != nil {
		return models.Tender{}, fmt.Errorf("service.Service.RollbackTender: %w", err)
	}

	// check whether user is employee of organization or not
	valid, err := s.repo.UserValid(ctx, user.Id, tender.OrganizationId)
	if err != nil {
		return models.Tender{}, fmt.Errorf("service.Service.RollbackTender: %w", err)
	}

	// if user is not employee of organization owning tender, return error
	if !valid {
		return models.Tender{}, models.ErrForbidden
	}

	// check version number
	if version < 1 || version > tender.Version {
		return models.Tender{}, models.ErrNoVersion
	}
	if version == tender.Version {
		return tender, nil
	}

	// check tender status
	if tender.Status == models.TenderClosed {
		return models.Tender{}, fmt.Errorf("service.Service.RollbackTender: %w", models.ErrTenderFinalized)
	}

	// find version and do rollback
	versions, err := s.repo.GetTenderVersions(ctx, tender.Id, version)
	if err != nil {
		return models.Tender{}, fmt.Errorf("service.Service.RollbackTender: %w", err)
	}
	if len(versions) == 0 {
		return models.Tender{}, models.ErrNoVersion
	}

	versions[0].Version = tender.Version
	err = s.repo.UpdateTender(ctx, versions[0], true)
	if err != nil {
		return models.Tender{}, fmt.Errorf("service.Service.RollbackTender: %w", err)
	}
	versions[0].Version++

	return versions[0], nil
}

//// Bids

func (s *Service) AddBid(ctx context.Context, bid models.Bid) (models.Bid, error) {
	// check if username exists
	err := s.setBidUserAndOrganization(ctx, &bid)
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.AddBid: %w", err)
	}

	// check if tender exists
	tender, err := s.repo.GetTenderByUUID(ctx, bid.TenderId, nil)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Bid{}, fmt.Errorf("service.Service.AddBid: %w", models.ErrNoTender)
	} else if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.AddBid: %w", err)
	}

	// check if tender is open for proposals
	if tender.Status != models.TenderPublished {
		return models.Bid{}, fmt.Errorf("service.Service.AddBid: %w", models.ErrNoTender)
	}

	// add bid
	bid, err = s.repo.AddBid(ctx, bid)
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.AddBid: %w", err)
	}

	return bid, nil
}

func (s *Service) GetUserBids(ctx context.Context, username string, limit, offset int) ([]models.Bid, error) {
	// check if username exists
	user, err := s.userByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("service.Service.GetUserBids: %w", err)
	}

	// get bids
	bids, err := s.repo.GetBids(ctx, limit, offset, user.Id, "")
	if err != nil {
		return nil, fmt.Errorf("service.Service.GetUserBids: %w", err)
	}

	return bids, nil
}

func (s *Service) GetTenderBids(ctx context.Context, username, tenderId string, limit, offset int) ([]models.Bid, error) {
	// check if username exists
	user, err := s.userByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("service.Service.GetTenderBids: %w", err)
	}

	// get tender
	tender, err := s.repo.GetTenderByUUID(ctx, tenderId, nil)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("service.Service.GetTenderBids: %w", models.ErrNoTender)
	} else if err != nil {
		return nil, fmt.Errorf("service.Service.GetTenderBids: %w", err)
	}

	// check whether user is employee of organization or not
	valid, err := s.repo.UserValid(ctx, user.Id, tender.OrganizationId)
	if err != nil {
		return nil, fmt.Errorf("service.Service.GetTenderBids: %w", err)
	}

	// if user is not employee and tender is not public, forbid access
	if !valid && tender.Status != models.TenderPublished {
		return nil, models.ErrForbidden
	}

	// get bids
	bids, err := s.repo.GetBids(ctx, limit, offset, "", tender.Id)
	if err != nil {
		return nil, fmt.Errorf("service.Service.GetTenderBids: %w", err)
	}

	return bids, nil
}

func (s *Service) GetBidStatus(ctx context.Context, username, bidId string) (models.BidStatus, error) {
	// check if username exists
	user, err := s.userByUsername(ctx, username)
	if err != nil {
		return "", fmt.Errorf("service.Service.GetBidStatus: %w", err)
	}

	// find bid
	bid, err := s.repo.GetBidByUUID(ctx, bidId)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("service.Service.GetBidStatus: %w", models.ErrNoBid)
	} else if err != nil {
		return "", fmt.Errorf("service.Service.GetBidStatus: %w", err)
	}

	if bid.AuthorId == user.Id {
		return bid.Status, nil
	}

	// find tender
	tender, err := s.repo.GetTenderByUUID(ctx, bid.TenderId, nil)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("service.Service.GetBidStatus: %w", models.ErrNoTender)
	} else if err != nil {
		return "", fmt.Errorf("service.Service.GetBidStatus: %w", err)
	}

	// check whether user is employee of organization or not
	valid, err := s.repo.UserValid(ctx, user.Id, tender.OrganizationId)
	if err != nil {
		return "", fmt.Errorf("service.Service.GetBidStatus: %w", err)
	}

	if valid || tender.Status == models.TenderPublished {
		return bid.Status, nil
	}

	return "", models.ErrForbidden
}

func (s *Service) SetBidStatus(ctx context.Context, username, bidId string, status models.BidStatus) (models.Bid, error) {
	// check if username exists
	user, err := s.userByUsername(ctx, username)
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.SetBidStatus: %w", err)
	}

	// find bid
	bid, err := s.repo.GetBidByUUID(ctx, bidId)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Bid{}, fmt.Errorf("service.Service.SetBidStatus: %w", models.ErrNoBid)
	} else if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.SetBidStatus: %w", err)
	}

	// check bid's status
	if bid.Status == models.BidApproved || bid.Status == models.BidRejected {
		return models.Bid{}, models.ErrBidFinalized
	}

	// check whether user is bid's author or employee of organization owning bid
	valid := false
	if status == models.BidApproved || status == models.BidRejected {
		// only for tender's owner
		tender, err := s.repo.GetTenderByUUID(ctx, bid.TenderId, nil)
		if errors.Is(err, sql.ErrNoRows) {
			return models.Bid{}, fmt.Errorf("service.Service.SetBidStatus: %w", models.ErrNoTender)
		} else if err != nil {
			return models.Bid{}, fmt.Errorf("service.Service.SetBidStatus: %w", err)
		}

		valid, err = s.repo.UserValid(ctx, user.Id, tender.OrganizationId)
		if err != nil {
			return models.Bid{}, fmt.Errorf("service.Service.SetBidStatus: %w", err)
		}

		if valid && status == models.BidApproved {
			m, err := s.repo.ApprovalCounts(ctx, bid.Id)
			if err != nil {
				return models.Bid{}, fmt.Errorf("service.Service.SetBidStatus: %w", err)
			}

			if m[models.ATReject] > 0 {
				return models.Bid{}, models.ErrBidFinalized
			}

			count, err := s.repo.EmployeeCount(ctx, tender.OrganizationId)
			if err != nil {
				return models.Bid{}, fmt.Errorf("service.Service.SetBidStatus: %w", err)
			}

			if !(m[models.ATApprove] >= 3 || m[models.ATApprove] >= count) {
				return models.Bid{}, models.ErrBidCannotBeApprovedYet
			}
		}
	}

	if !valid {
		valid, err = s.userAllowedToEditBid(ctx, user, bid)
		if err != nil {
			return models.Bid{}, fmt.Errorf("service.Service.SetBidStatus: %w", err)
		}
	}

	if !valid {
		return models.Bid{}, models.ErrForbidden
	}

	// update status
	bid.Status = status
	s.repo.UpdateBid(ctx, bid, true)
	bid.Version++
	return bid, nil
}

func (s *Service) EditBid(ctx context.Context, username, bidId string, changes map[string]string) (models.Bid, error) {
	// check if username exists
	user, err := s.userByUsername(ctx, username)
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.EditBid: %w", err)
	}

	// find bid
	bid, err := s.repo.GetBidByUUID(ctx, bidId)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Bid{}, fmt.Errorf("service.Service.EditBid: %w", models.ErrNoBid)
	} else if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.EditBid: %w", err)
	}

	// check bid's status
	if bid.Status == models.BidApproved || bid.Status == models.BidRejected {
		return models.Bid{}, models.ErrBidFinalized
	}

	valid, err := s.userAllowedToEditBid(ctx, user, bid)
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.EditBid: %w", err)
	}
	if !valid {
		return models.Bid{}, models.ErrForbidden
	}

	// remarshal changes into bid
	data, err := json.Marshal(changes)
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.EditBid: %w", err)
	}
	err = json.Unmarshal(data, &bid)
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.EditBid: %w", err)
	}

	// update bid
	err = s.repo.UpdateBid(ctx, bid, true)
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.EditBid: %w", err)
	}
	bid.Version++
	return bid, nil
}

func (s *Service) BidApproval(ctx context.Context, username, bidId string, status models.ApproveType) (models.Bid, error) {
	// check if username exists
	user, err := s.userByUsername(ctx, username)
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.BidApproval: %w", err)
	}

	// find bid
	bid, err := s.repo.GetBidByUUID(ctx, bidId)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Bid{}, fmt.Errorf("service.Service.BidApproval: %w", models.ErrNoBid)
	} else if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.BidApproval: %w", err)
	}

	// check bid's status
	if bid.Status == models.BidCreated || bid.Status == models.BidCanceled {
		return models.Bid{}, models.ErrForbidden
	}

	if bid.Status == models.BidApproved && status != models.ATApprove ||
		bid.Status == models.BidRejected && status != models.ATReject {
		return models.Bid{}, models.ErrBidFinalized
	}

	// ensure user has rights to approve bid (employee of organization owning tender)
	tender, err := s.repo.GetTenderByUUID(ctx, bid.TenderId, nil)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Bid{}, fmt.Errorf("service.Service.BidApproval: %w", models.ErrNoTender)
	} else if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.BidApproval: %w", err)
	}

	valid, err := s.repo.UserValid(ctx, user.Id, tender.OrganizationId)
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.BidApproval: %w", err)
	}
	if !valid {
		return models.Bid{}, models.ErrForbidden
	}

	// add approval
	err = s.repo.AddBidApproval(ctx, bidId, user.Id, status)
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.BidApproval: %w", err)
	}

	// count approvals and change bid / tender status
	counts, err := s.repo.ApprovalCounts(ctx, bid.Id)
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.BidApproval: %w", err)
	}

	if counts[models.ATReject] > 0 {
		// if at least 1 reject present, mark bid as rejected
		bid.Status = models.BidRejected
		err = s.repo.UpdateBid(ctx, bid, false)
		if err != nil {
			return models.Bid{}, fmt.Errorf("service.Service.BidApproval: %w", err)
		}

	} else if counts[models.ATApprove] > 0 {
		// if at least 1 approve present, check approve count against employee count and change statuses if necessary
		n, err := s.repo.EmployeeCount(ctx, tender.OrganizationId)
		if err != nil {
			return models.Bid{}, fmt.Errorf("service.Service.BidApproval: %w", err)
		}

		if counts[models.ATApprove] >= 3 || counts[models.ATApprove] >= n {
			bid.Status = models.BidApproved
			err = s.repo.UpdateBid(ctx, bid, false)
			if err != nil {
				return models.Bid{}, fmt.Errorf("service.Service.BidApproval: %w", err)
			}

			tender.Status = models.TenderClosed
			err = s.repo.UpdateTender(ctx, tender, false)
			if err != nil {
				return models.Bid{}, fmt.Errorf("service.Service.BidApproval: %w", err)
			}
		}
	}

	return bid, nil
}

func (s *Service) BidFeedback(ctx context.Context, username, bidId, feedback string) (models.Bid, error) {
	// check if username exists
	user, err := s.userByUsername(ctx, username)
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.BidFeedback: %w", err)
	}

	// find bid
	bid, err := s.repo.GetBidByUUID(ctx, bidId)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Bid{}, fmt.Errorf("service.Service.BidFeedback: %w", models.ErrNoBid)
	} else if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.BidFeedback: %w", err)
	}

	// find tender
	tender, err := s.repo.GetTenderByUUID(ctx, bid.TenderId, nil)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Bid{}, fmt.Errorf("service.Service.BidFeedback: %w", models.ErrNoTender)
	} else if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.BidFeedback: %w", err)
	}

	// if user is not employee of organization owning tender - forbid action
	valid, err := s.repo.UserValid(ctx, user.Id, tender.OrganizationId)
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.BidFeedback: %w", err)
	}
	if !valid {
		return models.Bid{}, models.ErrForbidden
	}

	err = s.repo.AddReview(ctx, models.BidReview{
		BidId:       bid.Id,
		UserId:      user.Id,
		Description: feedback,
	})
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.BidFeedback: %w", err)
	}

	return bid, nil
}

func (s *Service) BidRollback(ctx context.Context, username, bidId string, version int) (models.Bid, error) {
	// check if username exists
	user, err := s.userByUsername(ctx, username)
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.BidRollback: %w", err)
	}

	// find bid
	bid, err := s.repo.GetBidByUUID(ctx, bidId)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Bid{}, fmt.Errorf("service.Service.BidRollback: %w", models.ErrNoBid)
	} else if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.BidRollback: %w", err)
	}

	// check bid's status
	if bid.Status == models.BidApproved || bid.Status == models.BidRejected {
		return models.Bid{}, models.ErrBidFinalized
	}

	// check user's rights
	valid, err := s.userAllowedToEditBid(ctx, user, bid)
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.BidRollback: %w", err)
	}
	if !valid {
		return models.Bid{}, models.ErrForbidden
	}

	// find version
	versions, err := s.repo.GetBidVersions(ctx, bid.Id, version)
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.BidRollback: %w", err)
	}
	if len(versions) == 0 {
		return models.Bid{}, models.ErrNoVersion
	}

	// update bid
	versions[0].Version = bid.Version
	err = s.repo.UpdateBid(ctx, versions[0], true)
	if err != nil {
		return models.Bid{}, fmt.Errorf("service.Service.BidRollback: %w", err)
	}
	versions[0].Version++

	return versions[0], nil
}

func (s *Service) PastUserBidsReviews(ctx context.Context, tenderId, requesterName, authorName string, limit, offset int) ([]models.BidReview, error) {
	// check users
	requester, err := s.userByUsername(ctx, requesterName)
	if err != nil {
		return nil, fmt.Errorf("service.Service.PastUserBidsReviews: %w", err)
	}
	author, err := s.userByUsername(ctx, authorName)
	if err != nil {
		return nil, fmt.Errorf("service.Service.PastUserBidsReviews: %w", err)
	}

	// get tender
	tender, err := s.repo.GetTenderByUUID(ctx, tenderId, nil)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("service.Service.PastUserBidsReviews: %w", models.ErrNoTender)
	} else if err != nil {
		return nil, fmt.Errorf("service.Service.PastUserBidsReviews: %w", err)
	}

	// check if requester is actually employee of organization owning tender
	valid, err := s.repo.UserValid(ctx, requester.Id, tender.OrganizationId)
	if err != nil {
		return nil, fmt.Errorf("service.Service.PastUserBidsReviews: %w", err)
	}
	if !valid {
		return nil, models.ErrForbidden
	}

	// get reviews
	reviews, err := s.repo.GetReviews(ctx, limit, offset, "", "", author.Id)
	if err != nil {
		return nil, fmt.Errorf("service.Service.PastUserBidsReviews: %w", err)
	}
	return reviews, nil
}

//// Service

func (s *Service) userAllowedToEditBid(ctx context.Context, user models.User, bid models.Bid) (bool, error) {
	var err error

	valid := false
	if bid.AuthorId == user.Id {
		valid = true

	} else if bid.AuthorType == models.AuthorOrganization {
		valid, err = s.repo.UserValid(ctx, user.Id, bid.AuthorId)
		if err != nil {
			return false, fmt.Errorf("service.Service.userAllowedToEditBid: %w", err)
		}

	} else {
		valid, err = s.repo.UsersAreColleagues(ctx, user.Id, bid.AuthorId)
		if err != nil {
			return false, fmt.Errorf("service.Service.userAllowedToEditBid: %w", err)
		}
	}

	return valid, err
}

func (s *Service) setBidUserAndOrganization(ctx context.Context, bid *models.Bid) error {
	if bid.AuthorType == models.AuthorUser {
		bid.UserId = bid.AuthorId
		_, exist, err := s.repo.UserByUUID(ctx, bid.AuthorId)
		if err != nil {
			return fmt.Errorf("service.Service.setBidUserAndOrganization: %w", err)
		}
		if !exist {
			return models.ErrInvalidUser
		}
	} else if bid.AuthorType == models.AuthorOrganization {
		_, err := s.repo.OrganizationByUUID(ctx, bid.AuthorId)
		if err == sql.ErrNoRows {
			return models.ErrInvalidUser
		} else if err != nil {
			return fmt.Errorf("service.Service.setBidUserAndOrganization: %w", err)
		}
		bid.OrganizationId = bid.AuthorId
	}

	return nil
}

func (s *Service) userByUsername(ctx context.Context, username string) (models.User, error) {
	user, ok, err := s.repo.UserByUsername(ctx, username)
	if err != nil {
		return models.User{}, fmt.Errorf("service.Service.userByUsername: %w", err)
	}
	if !ok {
		return models.User{}, fmt.Errorf("service.Service.userByUsername: %w: %s", models.ErrInvalidUser, username)
	}
	return user, err
}
