// astra/controllers/user.go (new)
package controllers

import (
	"astra/astra/sources/psql/dao"
	"context"
)

type UserController struct {
	dao *dao.UserDAO
}

func NewUserController(dao *dao.UserDAO) *UserController {
	return &UserController{dao: dao}
}

func (c *UserController) GetUser(ctx context.Context, id int) (map[string]interface{}, error) {
	return c.dao.GetUserByID(ctx, id)
}

func (c *UserController) GetAllUsers(ctx context.Context) ([]map[string]interface{}, error) {
	return c.dao.GetAllUsers(ctx)
}

func (c *UserController) CreateUser(ctx context.Context, username, email string, fullName *string) (map[string]interface{}, error) {
	user, err := c.dao.CreateUser(ctx, username, email, fullName)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":        user.ID,
		"username":  user.Username,
		"email":     user.Email,
		"full_name": user.FullName,
	}, nil
}
