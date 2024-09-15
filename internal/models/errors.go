package models

import "errors"

var (
	ErrInvalidUser            = errors.New("provided user either does not exist or has no permission for this operation")
	ErrForbidden              = errors.New("provided user does not have permission for this operation")
	ErrNoTender               = errors.New("requestd tender does not exist")
	ErrTenderFinalized        = errors.New("tender is already closed")
	ErrNoBid                  = errors.New("requestd bid does not exist")
	ErrNoVersion              = errors.New("required version does not exist")
	ErrBidFinalized           = errors.New("bid is already approved or rejected")
	ErrBidCannotBeApprovedYet = errors.New("bid has not enough votes to be approved")
)
