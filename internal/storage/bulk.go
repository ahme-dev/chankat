package storage

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

func (s *Storage) bulkAssign(
	ctx context.Context,
	entity string,
	column string,
	value any,
	ids []int,
) error {
	ids, err := uniqueIDs(ids)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin bulk %s update: %w", entity, err)
	}
	defer tx.Rollback()

	countQuery, countArgs, err := sqlx.In(
		fmt.Sprintf(`SELECT count(*) FROM %s WHERE ID IN (?)`, entity),
		ids,
	)
	if err != nil {
		return fmt.Errorf("build bulk %s count: %w", entity, err)
	}

	var count int
	if err := tx.GetContext(ctx, &count, countQuery, countArgs...); err != nil {
		return fmt.Errorf("count %s: %w", entity, err)
	}
	if count != len(ids) {
		return fmt.Errorf("found %d of %d requested %s records", count, len(ids), entity)
	}

	updateQuery, updateArgs, err := sqlx.In(
		fmt.Sprintf(`UPDATE %s SET %s = ? WHERE ID IN (?)`, entity, column),
		value,
		ids,
	)
	if err != nil {
		return fmt.Errorf("build bulk %s update: %w", entity, err)
	}
	if _, err := tx.ExecContext(ctx, updateQuery, updateArgs...); err != nil {
		return fmt.Errorf("bulk update %s: %w", entity, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit bulk %s update: %w", entity, err)
	}
	return nil
}

func uniqueIDs(ids []int) ([]int, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("expected at least one ID")
	}

	unique := make([]int, 0, len(ids))
	seen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("invalid ID %d", id)
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique, nil
}
