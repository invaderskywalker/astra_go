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

// UpdateUser updates fields of the user by id. If a field is nil, it is not updated.
func (c *UserController) UpdateUser(ctx context.Context, id int, username, email, fullName, imageURL *string) (*models.User, error) {
	user, err := c.dao.GetUserByID(ctx, id)
	if err != nil || user == nil {
		return nil, err
	}
	if username != nil {
		user.Username = *username
	}
	if email != nil {
		user.Email = *email
	}
	if fullName != nil {
		user.FullName = fullName
	}
	if imageURL != nil {
		user.ImageURL = imageURL
	}
	if err := c.dao.UpdateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (c *UserController) CreateUser(ctx context.Context, username, email string, fullName *string, imageURL *string) (*models.User, error) {
	return c.dao.CreateUser(ctx, username, email, fullName, imageURL)
}
