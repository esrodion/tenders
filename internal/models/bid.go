package models

import "time"

type AuthorType string

const (
	AuthorUser         AuthorType = "User"
	AuthorOrganization AuthorType = "Organization"
)

func ValidAuthorType(t AuthorType) bool {
	switch t {
	case AuthorUser, AuthorOrganization:
		return true
	default:
		return false
	}
}

type BidStatus string

const (
	BidCreated   BidStatus = "Created"
	BidPublished BidStatus = "Published"
	BidCanceled  BidStatus = "Canceled"
	BidApproved  BidStatus = "Approved"
	BidRejected  BidStatus = "Rejected"
)

func ValidBidStatus(t BidStatus) bool {
	switch t {
	case BidCreated, BidPublished, BidCanceled, BidApproved, BidRejected:
		return true
	default:
		return false
	}
}

type Bid struct {
	Id             string     `json:"id"`
	Version        int        `json:"version"`
	TenderId       string     `json:"tenderId"`
	AuthorType     AuthorType `json:"authorType"`
	AuthorId       string     `json:"authorId"`
	UserId         string     `json:"-"`
	OrganizationId string     `json:"-"`
	Status         BidStatus  `json:"status"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"-"`
}
