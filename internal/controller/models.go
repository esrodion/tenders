package controller

import (
	"encoding/json"
	"fmt"
	"tenders/internal/models"
)

// New tender request

type NewTenderReq struct {
	Name           string              `json:"name"`
	Description    string              `json:"description"`
	ServiceType    models.ServiceType  `json:"serviceType"`
	Status         models.TenderStatus `json:"status"`
	OrganizationId string              `json:"organizationId"`
	AuthorUsername string              `json:"creatorUsername"`
}

func ParseNewTenderReq(data []byte) (*NewTenderReq, error) {
	t := &NewTenderReq{}

	err := json.Unmarshal(data, t)
	if err != nil {
		return nil, err
	}

	if !models.ValidServiceType(t.ServiceType) {
		return nil, fmt.Errorf("invalid service type supplied: %s, should be one of: %s, %s, %s", string(t.ServiceType), models.STConstruction, models.STDelivery, models.STManufacture)
	}

	if len(t.Status) == 0 {
		t.Status = models.TenderCreated
	} else if !models.ValidTenderStatus(t.Status) {
		return nil, fmt.Errorf("invalid tender status supplied: %s, should be one of: %s, %s, %s", string(t.Status), models.TenderCreated, models.TenderPublished, models.TenderClosed)
	}

	if err = checkLengthLimit(t.Name, "Name", 100); err != nil {
		return nil, err
	}
	if err = checkLengthLimit(t.OrganizationId, "OrganizationId", 100); err != nil {
		return nil, err
	}
	if err = checkLengthLimit(t.Description, "Description", 500); err != nil {
		return nil, err
	}

	return t, nil
}

// Edit tender request

type TenderChangeReq map[string]string

func ParseTenderChangeReq(data []byte) (TenderChangeReq, error) {
	t := TenderChangeReq{}
	vals := make(map[string]interface{})

	err := json.Unmarshal(data, &vals)
	if err != nil {
		return nil, err
	}

	str, ok, err := checkRequestField(vals, "serviceType", 100)
	if err != nil {
		return nil, err
	}
	if !models.ValidServiceType(models.ServiceType(str)) {
		return nil, fmt.Errorf("invalid service type supplied: %s", str)
	}
	if ok {
		t["serviceType"] = str
	}

	str, ok, err = checkRequestField(vals, "name", 100)
	if err != nil {
		return nil, err
	}
	if ok {
		t["name"] = str
	}

	str, ok, err = checkRequestField(vals, "description", 100)
	if err != nil {
		return nil, err
	}
	if ok {
		t["description"] = str
	}

	return t, nil
}

// New bid request

type NewBidReq struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	TenderId    string            `json:"tenderId"`
	AuthorType  models.AuthorType `json:"authorType"`
	AuthorId    string            `json:"authorId"`
}

func ParseNewBidReq(data []byte) (*NewBidReq, error) {
	t := &NewBidReq{}

	err := json.Unmarshal(data, t)
	if err != nil {
		return nil, err
	}

	if !models.ValidAuthorType(t.AuthorType) {
		return nil, fmt.Errorf("invalid service type supplied: %s", t.AuthorType)
	}
	if err = checkLengthLimit(t.Name, "Name", 100); err != nil {
		return nil, err
	}
	if err = checkLengthLimit(t.TenderId, "TenderId", 100); err != nil {
		return nil, err
	}
	if err = checkLengthLimit(t.AuthorId, "AuthorId", 100); err != nil {
		return nil, err
	}
	if err = checkLengthLimit(t.Description, "Description", 500); err != nil {
		return nil, err
	}

	return t, nil
}

// Service

func checkLengthLimit(str, fieldName string, limit int) error {
	if len(str) > limit {
		return fmt.Errorf("field '%s' exceeds length limit: %d / %d", fieldName, len(str), 100)
	}
	return nil
}

func checkRequestField(vals map[string]interface{}, key string, lentghLimit int) (string, bool, error) {
	val, ok := vals[key]
	if !ok {
		return "", false, nil
	}

	str, ok := val.(string)
	if !ok {
		return "", false, fmt.Errorf("invalid type od '%s' field", key)
	}

	if err := checkLengthLimit(str, key, lentghLimit); err != nil {
		return "", false, err
	}

	return str, true, nil
}
