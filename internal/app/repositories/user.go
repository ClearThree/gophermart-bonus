package repositories

import (
	"context"
	"database/sql"
	"errors"
	"github.com/ClearThree/gophermart-bonus/internal/app/logger"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

type User struct {
	ID       uint64
	Login    string
	Password string
}

type UserRepositoryInterface interface {
	Create(ctx context.Context, login string, password string) (User, error)
	Read(ctx context.Context, login string) (User, error)
	GetBalances(ctx context.Context, userID uint64) (float32, float32, error)
}

var ErrLoginAlreadyTaken = errors.New("login already taken")
var ErrUserNotFound = errors.New("no user found with the given login")

type UserRepository struct {
	pool *sql.DB
}

func NewUserRepository(pool *sql.DB) *UserRepository {
	return &UserRepository{pool: pool}
}

func (u UserRepository) Create(ctx context.Context, login string, password string) (User, error) {
	transaction, txErr := u.pool.BeginTx(ctx, nil)
	if txErr != nil {
		return User{}, txErr
	}
	createUserPreparedStmt, err := transaction.PrepareContext(
		ctx, `INSERT INTO "user" (login, password) VALUES ($1, $2) RETURNING id, login, password`)
	if err != nil {
		return User{}, err
	}
	row := createUserPreparedStmt.QueryRowContext(ctx, login, password)
	var ID uint64
	var selectedLogin string
	var selectedPassword string
	err = row.Scan(&ID, &selectedLogin, &selectedPassword)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgerrcode.IsIntegrityConstraintViolation(pgErr.Code) {
			logger.Log.Infof("Login %s already taken", login)
			return User{}, ErrLoginAlreadyTaken
		}
		txErr = transaction.Rollback()
		if txErr != nil {
			return User{}, txErr
		}
		return User{}, err
	}
	createUserBalancePreparedStmt, err := transaction.PrepareContext(
		ctx, `INSERT INTO "user-balance" (user_id) VALUES ($1)`)
	if err != nil {
		return User{}, err
	}
	_, err = createUserBalancePreparedStmt.ExecContext(ctx, ID)
	if err != nil {
		txErr = transaction.Rollback()
		if txErr != nil {
			return User{}, txErr
		}
		return User{}, err
	}
	txErr = transaction.Commit()
	if txErr != nil {
		return User{}, txErr
	}
	return User{
		ID:       uint64(ID),
		Login:    login,
		Password: password,
	}, nil
}

func (u UserRepository) Read(ctx context.Context, login string) (User, error) {
	readUserByLoginPreparedStmt, err := u.pool.PrepareContext(
		ctx, `SELECT id, login, password FROM "user" where login = $1 and active`)
	if err != nil {
		return User{}, err
	}
	row := readUserByLoginPreparedStmt.QueryRowContext(ctx, login)
	var ID uint64
	var selectedLogin string
	var password string
	err = row.Scan(&ID, &selectedLogin, &password)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, err
	}
	if row == nil {
		return User{}, ErrUserNotFound
	}
	user := User{
		ID:       uint64(ID),
		Login:    login,
		Password: password,
	}
	return user, nil
}

func (u UserRepository) GetBalances(ctx context.Context, userID uint64) (float32, float32, error) {
	getUserBalancePreparedStmt, err := u.pool.PrepareContext(
		ctx, `SELECT balance, withdrawals_sum FROM "user-balance" where user_id = $1`)
	if err != nil {
		return 0.0, 0.0, err
	}
	row := getUserBalancePreparedStmt.QueryRowContext(ctx, userID)
	var balance float32
	var withdrawalsSum float32
	err = row.Scan(&balance, &withdrawalsSum)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0.0, 0.0, ErrUserNotFound
		}
		return 0.0, 0.0, err
	}
	return balance, withdrawalsSum, nil
}
