package domain

import "encoding/json"

func CloneBatch(in *DendroBatch) (*DendroBatch, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out DendroBatch
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
