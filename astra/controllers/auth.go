// astra/controllers/auth.go (new folder and file)
package controllers

import (
	"astra/astra/sources/psql/dao"
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"astra/astra/config"
)

type AuthController struct {
	userDAO *dao.UserDAO
}

func NewAuthController(userDAO *dao.UserDAO) *AuthController {
	return &AuthController{userDAO: userDAO}
}

func (c *AuthController) Login(ctx context.Context, username string) (string, error) {
	user, err := c.userDAO.GetUserByUsername(ctx, username)
	if err != nil {
		return "", err
	}
	if user == nil {
		// Auto-create with dummy email
		email := username + "@example.com"
		user, err = c.userDAO.CreateUser(ctx, username, email, nil)
		if err != nil {
			return "", err
		}
	}
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.LoadConfig().JWT_SECRET))
}
