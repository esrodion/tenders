package models

import "time"

type ApproveType string

const (
	ATApprove ApproveType = "Approved"
	ATReject  ApproveType = "Rejected"
)

func ValidApproveType(t ApproveType) bool {
	switch t {
	case ATApprove, ATReject:
		return true
	default:
		return false
	}
}

type BidApproval struct {
	BidId     string
	UserId    string
	Status    ApproveType
	CreatedAt time.Time
	UpdatedAt time.Time
}

type BidReview struct {
	BidId       string
	UserId      string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
