package models

import "time"

type TenderStatus string

const (
	TenderCreated   TenderStatus = "Created"
	TenderPublished TenderStatus = "Published"
	TenderClosed    TenderStatus = "Closed"
)

func ValidTenderStatus(t TenderStatus) bool {
	switch t {
	case TenderCreated, TenderPublished, TenderClosed:
		return true
	default:
		return false
	}
}

type ServiceType string

const (
	STConstruction ServiceType = "Construction"
	STDelivery     ServiceType = "Delivery"
	STManufacture  ServiceType = "Manufacture"
)

func ValidServiceType(t ServiceType) bool {
	switch t {
	case STConstruction, STDelivery, STManufacture:
		return true
	default:
		return false
	}
}

type Tender struct {
	Id             string       `json:"id"`
	Version        int          `json:"version"`
	OrganizationId string       `json:"organizationId"`
	Author         string       `json:"-"`
	Status         TenderStatus `json:"status"`
	ServiceType    ServiceType  `json:"serviceType"`
	Name           string       `json:"name"`
	Description    string       `json:"description"`
	CreatedAt      time.Time    `json:"createdAt"`
	UpdatedAt      time.Time    `json:"-"`
}
