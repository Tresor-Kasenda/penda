package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"penda/framework/app"
	fwctx "penda/framework/context"
	fwerrors "penda/framework/errors"
	"penda/framework/middleware"
	"penda/framework/observability"
	"penda/framework/orm"
)

type User struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name"`
	Email     string    `json:"email" gorm:"uniqueIndex"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type createUserInput struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required"`
}

type updateUserInput struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
}

func BuildApp(db *gorm.DB) (*app.App, error) {
	if err := orm.AutoMigrate(db, &User{}); err != nil {
		return nil, err
	}

	server := app.New()
	metrics := observability.NewMetrics()
	server.Use(
		middleware.Recovery(),
		middleware.RequestID(),
		middleware.Logger(nil),
		metrics.Middleware(),
		orm.Middleware(db),
	)

	server.Get("/health", observability.HealthHandler())
	server.Get("/ready", observability.ReadinessHandler(func() error {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Ping()
	}))
	server.Get("/metrics", metrics.Handler())

	api := server.Group("/api")
	api.Get("/users", listUsers)
	api.Post("/users", createUser)
	api.Get("/users/:id", getUser)
	api.Patch("/users/:id", updateUser)
	api.Delete("/users/:id", deleteUser)

	return server, nil
}

func listUsers(c *fwctx.Context) error {
	db, ok := orm.FromContext(c)
	if !ok {
		return fwerrors.Internal("database is not available", nil)
	}

	var users []User
	if err := db.Order("id ASC").Find(&users).Error; err != nil {
		return fwerrors.Internal("failed to list users", err)
	}

	return c.JSON(http.StatusOK, users)
}

func createUser(c *fwctx.Context) error {
	db, ok := orm.FromContext(c)
	if !ok {
		return fwerrors.Internal("database is not available", nil)
	}

	var input createUserInput
	if err := c.BindJSON(&input); err != nil {
		return err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.TrimSpace(input.Email)
	if input.Name == "" || input.Email == "" {
		return fwerrors.BadRequest("name and email are required", nil)
	}

	user := User{Name: input.Name, Email: input.Email}
	if err := db.Create(&user).Error; err != nil {
		return fwerrors.Conflict("failed to create user", err)
	}

	return c.JSON(http.StatusCreated, user)
}

func getUser(c *fwctx.Context) error {
	db, ok := orm.FromContext(c)
	if !ok {
		return fwerrors.Internal("database is not available", nil)
	}

	id, err := parseUserID(c.Param("id"))
	if err != nil {
		return err
	}

	var user User
	if err := db.First(&user, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fwerrors.NotFound("user not found", err)
		}
		return fwerrors.Internal("failed to fetch user", err)
	}

	return c.JSON(http.StatusOK, user)
}

func updateUser(c *fwctx.Context) error {
	db, ok := orm.FromContext(c)
	if !ok {
		return fwerrors.Internal("database is not available", nil)
	}

	id, err := parseUserID(c.Param("id"))
	if err != nil {
		return err
	}

	var input updateUserInput
	if err := c.BindJSON(&input); err != nil {
		return err
	}

	updates := map[string]any{}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return fwerrors.BadRequest("name cannot be empty", nil)
		}
		updates["name"] = name
	}
	if input.Email != nil {
		email := strings.TrimSpace(*input.Email)
		if email == "" {
			return fwerrors.BadRequest("email cannot be empty", nil)
		}
		updates["email"] = email
	}
	if len(updates) == 0 {
		return fwerrors.BadRequest("no fields to update", nil)
	}

	result := db.Model(&User{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fwerrors.Conflict("failed to update user", result.Error)
	}
	if result.RowsAffected == 0 {
		return fwerrors.NotFound("user not found", nil)
	}

	var user User
	if err := db.First(&user, id).Error; err != nil {
		return fwerrors.Internal("failed to fetch updated user", err)
	}

	return c.JSON(http.StatusOK, user)
}

func deleteUser(c *fwctx.Context) error {
	db, ok := orm.FromContext(c)
	if !ok {
		return fwerrors.Internal("database is not available", nil)
	}

	id, err := parseUserID(c.Param("id"))
	if err != nil {
		return err
	}

	result := db.Delete(&User{}, id)
	if result.Error != nil {
		return fwerrors.Internal("failed to delete user", result.Error)
	}
	if result.RowsAffected == 0 {
		return fwerrors.NotFound("user not found", nil)
	}

	c.Status(http.StatusNoContent)
	return nil
}

func parseUserID(raw string) (uint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fwerrors.BadRequest("user id is required", nil)
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return 0, fwerrors.BadRequest(fmt.Sprintf("invalid user id %q", raw), err)
	}
	return uint(value), nil
}
