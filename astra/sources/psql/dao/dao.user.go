package dao

import (
	models "astra/astra/sources/psql/model"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserDAO struct {
	DB *pgxpool.Pool
}

func NewUserDAO(db *pgxpool.Pool) *UserDAO {
	return &UserDAO{DB: db}
}

func (dao *UserDAO) GetUserByID(ctx context.Context, id int) (map[string]interface{}, error) {
	query := "SELECT id, username, email, full_name FROM users WHERE id = $1"
	row := dao.DB.QueryRow(ctx, query, id)
	var user models.User
	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.FullName)
	if err != nil {
		return map[string]interface{}{"error": "user not found"}, nil
	}
	return map[string]interface{}{
		"id":        user.ID,
		"username":  user.Username,
		"email":     user.Email,
		"full_name": user.FullName,
	}, nil
}

func (dao *UserDAO) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	query := "SELECT id, username, email, full_name FROM users WHERE username = $1"
	row := dao.DB.QueryRow(ctx, query, username)
	var user models.User
	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.FullName)
	if err != nil {
		return nil, nil
	}
	return &user, nil
}

func (dao *UserDAO) CreateUser(ctx context.Context, username, email string, fullName *string) (*models.User, error) {
	query := "INSERT INTO users (username, email, full_name) VALUES ($1, $2, $3) RETURNING id, username, email, full_name"
	row := dao.DB.QueryRow(ctx, query, username, email, fullName)
	var user models.User
	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.FullName)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (dao *UserDAO) GetAllUsers(ctx context.Context) ([]map[string]interface{}, error) {
	query := "SELECT id, username, email, full_name FROM users"
	rows, err := dao.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []map[string]interface{}
	for rows.Next() {
		var user models.User
		err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.FullName)
		if err != nil {
			return nil, err
		}
		users = append(users, map[string]interface{}{
			"id":        user.ID,
			"username":  user.Username,
			"email":     user.Email,
			"full_name": user.FullName,
		})
	}
	return users, nil
}
