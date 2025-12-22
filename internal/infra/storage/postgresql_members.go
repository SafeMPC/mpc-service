package storage

import (
	"context"
	"database/sql"

	"github.com/pkg/errors"
)

// AddWalletMember 添加钱包成员
func (s *PostgreSQLStore) AddWalletMember(ctx context.Context, walletID, credentialID, role string) error {
	query := `
		INSERT INTO wallet_members (wallet_id, credential_id, role, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (wallet_id, credential_id) DO UPDATE SET
			role = EXCLUDED.role
	`
	_, err := s.db.ExecContext(ctx, query, walletID, credentialID, role)
	if err != nil {
		return errors.Wrap(err, "failed to add wallet member")
	}
	return nil
}

// RemoveWalletMember 移除钱包成员
func (s *PostgreSQLStore) RemoveWalletMember(ctx context.Context, walletID, credentialID string) error {
	query := `DELETE FROM wallet_members WHERE wallet_id = $1 AND credential_id = $2`
	_, err := s.db.ExecContext(ctx, query, walletID, credentialID)
	if err != nil {
		return errors.Wrap(err, "failed to remove wallet member")
	}
	return nil
}

// IsWalletMember 检查是否为钱包成员
func (s *PostgreSQLStore) IsWalletMember(ctx context.Context, walletID, credentialID string) (bool, string, error) {
	query := `SELECT role FROM wallet_members WHERE wallet_id = $1 AND credential_id = $2`
	var role string
	err := s.db.QueryRowContext(ctx, query, walletID, credentialID).Scan(&role)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, "", nil
		}
		return false, "", errors.Wrap(err, "failed to check wallet member")
	}
	return true, role, nil
}

// ListWalletMembers 列出钱包成员
func (s *PostgreSQLStore) ListWalletMembers(ctx context.Context, walletID string) ([]string, error) {
	query := `SELECT credential_id FROM wallet_members WHERE wallet_id = $1`
	rows, err := s.db.QueryContext(ctx, query, walletID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list wallet members")
	}
	defer rows.Close()

	var members []string
	for rows.Next() {
		var credID string
		if err := rows.Scan(&credID); err != nil {
			return nil, errors.Wrap(err, "failed to scan wallet member")
		}
		members = append(members, credID)
	}
	return members, nil
}
