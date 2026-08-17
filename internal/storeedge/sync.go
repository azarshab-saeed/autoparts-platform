package storeedge

import (
	"context"
	"fmt"
)

type Syncer struct {
	store *Store
	cloud *Cloud
}

func NewSyncer(store *Store, cloud *Cloud) *Syncer { return &Syncer{store: store, cloud: cloud} }

func (s *Syncer) Run(ctx context.Context) error {
	if !s.store.IsPaired() {
		return nil
	}
	cfg := s.store.Config()
	for _, sale := range s.store.PendingSales() {
		out, status, err := s.cloud.PushSale(ctx, cfg, sale)
		if err != nil {
			if status >= 400 && status < 500 {
				_ = s.store.MarkAttempt(sale.LocalOperationID, "", "conflict", err.Error())
				continue
			}
			_ = s.store.MarkAttempt(sale.LocalOperationID, "", "pending", err.Error())
			_ = s.store.SetSyncResult(err.Error())
			return err
		}
		if err := s.store.MarkAttempt(sale.LocalOperationID, out.ID, "synced", ""); err != nil {
			return err
		}
	}
	snapshot, err := s.cloud.Snapshot(ctx, cfg)
	if err != nil {
		_ = s.store.SetSyncResult(err.Error())
		return fmt.Errorf("refresh snapshot: %w", err)
	}
	if err := s.store.ReplaceSnapshot(snapshot); err != nil {
		return err
	}
	return s.store.SetSyncResult("")
}
