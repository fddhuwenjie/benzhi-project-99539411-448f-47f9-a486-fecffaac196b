package workflow

import (
	"strings"

	"dendro-chronology-workbench/internal/domain"
)

type BatchFilter struct {
	State      domain.State
	SiteCode   string
	Species    string
	OperatorID string
	BatchID    string
}

type BatchOverview struct {
	*domain.DendroBatch
	Progress domain.BatchProgress `json:"progress"`
}

type BatchOverviewResult struct {
	Batches    []BatchOverview        `json:"batches"`
	Statistics domain.BatchStatistics `json:"statistics"`
}

func (s *Service) SearchBatches(filter BatchFilter) (BatchOverviewResult, error) {
	batches, err := s.store.List()
	if err != nil {
		return BatchOverviewResult{}, err
	}
	matched := make([]*domain.DendroBatch, 0, len(batches))
	for _, batch := range batches {
		if filter.State != "" && batch.State != filter.State {
			continue
		}
		if !containsFold(batch.SiteCode, filter.SiteCode) {
			continue
		}
		if !containsFold(batch.Species, filter.Species) {
			continue
		}
		if !containsFold(batch.OperatorID, filter.OperatorID) {
			continue
		}
		if !containsFold(batch.BatchID, filter.BatchID) {
			continue
		}
		matched = append(matched, batch)
	}
	domain.SortBatchesForOverview(matched)
	result := BatchOverviewResult{Batches: make([]BatchOverview, 0, len(matched)), Statistics: domain.SummarizeBatches(matched)}
	for _, batch := range matched {
		result.Batches = append(result.Batches, BatchOverview{DendroBatch: batch, Progress: domain.Progress(batch)})
	}
	return result, nil
}

func containsFold(value, keyword string) bool {
	return keyword == "" || strings.Contains(strings.ToLower(value), strings.ToLower(keyword))
}
