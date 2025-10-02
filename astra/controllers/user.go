// astra/controllers/user.go
package controllers

import (
	"astra/astra/sources/psql/dao"
	"astra/astra/sources/psql/models"
	"context"
)

type UserController struct {
	dao *dao.UserDAO
}

func NewUserController(dao *dao.UserDAO) *UserController {
	return &UserController{dao: dao}
}

func (c *UserController) GetUser(ctx context.Context, id int) (*models.User, error) {
	return c.dao.GetUserByID(ctx, id)
}

func (c *UserController) GetAllUsers(ctx context.Context) ([]models.User, error) {
	return c.dao.GetAllUsers(ctx)
}

func (c *UserController) CreateUser(ctx context.Context, username, email string, fullName *string) (*models.User, error) {
	return c.dao.CreateUser(ctx, username, email, fullName)
}
