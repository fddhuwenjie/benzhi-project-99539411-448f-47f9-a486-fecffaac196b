package snapshot_cache_replacement_test

import (
	"testing"
	"time"

	"dendro-chronology-workbench/internal/domain"
	"dendro-chronology-workbench/internal/repository"
	"dendro-chronology-workbench/internal/workflow"
)

func TestSnapshotCacheObservesExternalAtomicReplace(t *testing.T) {
	root := t.TempDir()
	storeA, err := repository.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	serviceA := workflow.New(storeA)
	now := time.Now().UTC()
	_, err = serviceA.Create(workflow.CreateBatchCommand{
		CommandMeta: workflow.CommandMeta{RequestID: "cache-create", ActorID: "operator-cache"},
		BatchID:     "batch-cache-replace",
		SiteCode:    "SITE-CACHE",
		Species:     "Pinus",
		SampledAt:   now.Add(-time.Hour),
		OperatorID:  "operator-cache",
		Cores: []domain.CoreSample{{
			CoreID: "core-cache", TreeCode: "tree-cache", RadiusCode: "A",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := serviceA.Get("batch-cache-replace")
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != 1 || first.State != domain.StateBaselined {
		t.Fatalf("unexpected initial snapshot: revision=%d state=%s", first.Revision, first.State)
	}

	storeB, err := repository.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	serviceB := workflow.New(storeB)
	_, err = serviceB.RegisterImages("batch-cache-replace", workflow.RegisterImagesCommand{
		CommandMeta: workflow.CommandMeta{
			RequestID: "cache-images", ExpectedRevision: 1, ActorID: "operator-cache",
		},
		Images: []workflow.ImageInput{{
			CoreID:            "core-cache",
			PreparationMethod: "精磨",
			ImageDigest:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			MicronsPerPixel:   2,
			CapturedAt:        now,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	latest, err := serviceA.Get("batch-cache-replace")
	if err != nil {
		t.Fatal(err)
	}
	if latest.Revision != 2 || latest.State != domain.StateImaged {
		t.Fatalf("stale cached snapshot after external atomic replace: revision=%d state=%s", latest.Revision, latest.State)
	}
}
