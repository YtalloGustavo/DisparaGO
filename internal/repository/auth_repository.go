package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	authdomain "disparago/internal/domain/auth"
	"disparago/internal/platform/database"
)

var (
	ErrAuthUserNotFound = errors.New("auth user not found")
)

type AuthRepository struct {
	db *database.Client
}

type UpsertCompanyParams struct {
	Name           string
	ExternalSource string
	ExternalID     string
}

type UpsertUserParams struct {
	CompanyID      *int64
	Username       string
	DisplayName    string
	PasswordHash   string
	Role           authdomain.Role
	Active         bool
	ExternalSource string
	ExternalID     string
}

func NewAuthRepository(db *database.Client) *AuthRepository {
	return &AuthRepository{db: db}
}

func (r *AuthRepository) FindUserByUsername(ctx context.Context, username string) (authdomain.User, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT u.id, u.company_id, u.username, u.display_name, u.password_hash, u.role, u.active,
		       COALESCE(u.external_source, ''), COALESCE(u.external_source_id, ''), u.created_at, u.updated_at
		FROM auth_users u
		WHERE u.username = $1
	`, username)

	var user authdomain.User
	if err := row.Scan(
		&user.ID,
		&user.CompanyID,
		&user.Username,
		&user.DisplayName,
		&user.PasswordHash,
		&user.Role,
		&user.Active,
		&user.ExternalSource,
		&user.ExternalID,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return authdomain.User{}, ErrAuthUserNotFound
		}
		return authdomain.User{}, fmt.Errorf("query auth user: %w", err)
	}

	return user, nil
}

func (r *AuthRepository) FindCompanyByID(ctx context.Context, companyID int64) (authdomain.Company, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, name, COALESCE(external_source, ''), COALESCE(external_source_id, ''), created_at, updated_at
		FROM auth_companies
		WHERE id = $1
	`, companyID)

	var company authdomain.Company
	if err := row.Scan(
		&company.ID,
		&company.Name,
		&company.ExternalSource,
		&company.ExternalID,
		&company.CreatedAt,
		&company.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return authdomain.Company{}, ErrAuthUserNotFound
		}
		return authdomain.Company{}, fmt.Errorf("query company: %w", err)
	}

	return company, nil
}

func (r *AuthRepository) UpsertCompany(ctx context.Context, params UpsertCompanyParams) (authdomain.Company, error) {
	var company authdomain.Company

	if params.ExternalSource != "" && params.ExternalID != "" {
		row := r.db.Pool.QueryRow(ctx, `
			UPDATE auth_companies
			SET name = $1, updated_at = NOW()
			WHERE external_source = $2 AND external_source_id = $3
			RETURNING id, name, COALESCE(external_source, ''), COALESCE(external_source_id, ''), created_at, updated_at
		`, params.Name, params.ExternalSource, params.ExternalID)
		if err := row.Scan(
			&company.ID,
			&company.Name,
			&company.ExternalSource,
			&company.ExternalID,
			&company.CreatedAt,
			&company.UpdatedAt,
		); err == nil {
			return company, nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return authdomain.Company{}, fmt.Errorf("update company: %w", err)
		}
	}

	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO auth_companies (name, external_source, external_source_id, created_at, updated_at)
		VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), NOW(), NOW())
		RETURNING id, name, COALESCE(external_source, ''), COALESCE(external_source_id, ''), created_at, updated_at
	`, params.Name, params.ExternalSource, params.ExternalID)

	if err := row.Scan(
		&company.ID,
		&company.Name,
		&company.ExternalSource,
		&company.ExternalID,
		&company.CreatedAt,
		&company.UpdatedAt,
	); err != nil {
		return authdomain.Company{}, fmt.Errorf("insert company: %w", err)
	}

	return company, nil
}

func (r *AuthRepository) UpsertUser(ctx context.Context, params UpsertUserParams) (authdomain.User, error) {
	var user authdomain.User

	if params.ExternalSource != "" && params.ExternalID != "" {
		row := r.db.Pool.QueryRow(ctx, `
			UPDATE auth_users
			SET company_id = $1,
			    username = $2,
			    display_name = $3,
			    password_hash = $4,
			    role = $5,
			    active = $6,
			    updated_at = NOW()
			WHERE external_source = $7 AND external_source_id = $8
			RETURNING id, company_id, username, display_name, password_hash, role, active,
			          COALESCE(external_source, ''), COALESCE(external_source_id, ''), created_at, updated_at
		`,
			params.CompanyID,
			params.Username,
			params.DisplayName,
			params.PasswordHash,
			params.Role,
			params.Active,
			params.ExternalSource,
			params.ExternalID,
		)

		if err := row.Scan(
			&user.ID,
			&user.CompanyID,
			&user.Username,
			&user.DisplayName,
			&user.PasswordHash,
			&user.Role,
			&user.Active,
			&user.ExternalSource,
			&user.ExternalID,
			&user.CreatedAt,
			&user.UpdatedAt,
		); err == nil {
			return user, nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return authdomain.User{}, fmt.Errorf("update user by external ref: %w", err)
		}
	}

	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO auth_users (
			company_id, username, display_name, password_hash, role, active, external_source, external_source_id, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''), NOW(), NOW()
		)
		ON CONFLICT (username)
		DO UPDATE SET
		    company_id = EXCLUDED.company_id,
		    display_name = EXCLUDED.display_name,
		    password_hash = EXCLUDED.password_hash,
		    role = EXCLUDED.role,
		    active = EXCLUDED.active,
		    external_source = COALESCE(EXCLUDED.external_source, auth_users.external_source),
		    external_source_id = COALESCE(EXCLUDED.external_source_id, auth_users.external_source_id),
		    updated_at = NOW()
		RETURNING id, company_id, username, display_name, password_hash, role, active,
		          COALESCE(external_source, ''), COALESCE(external_source_id, ''), created_at, updated_at
	`,
		params.CompanyID,
		params.Username,
		params.DisplayName,
		params.PasswordHash,
		params.Role,
		params.Active,
		params.ExternalSource,
		params.ExternalID,
	)

	if err := row.Scan(
		&user.ID,
		&user.CompanyID,
		&user.Username,
		&user.DisplayName,
		&user.PasswordHash,
		&user.Role,
		&user.Active,
		&user.ExternalSource,
		&user.ExternalID,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		return authdomain.User{}, fmt.Errorf("upsert user by username: %w", err)
	}

	return user, nil
}
