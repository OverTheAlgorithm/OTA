package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"ota/domain/user"
)

// RoleChangeRepoForHandler is the slice of user.RoleChangeRepository the
// handler depends on. Declared here so tests can substitute a fake without
// importing the storage package.
type RoleChangeRepoForHandler interface {
	Log(ctx context.Context, entry user.RoleChangeLog) (user.RoleChangeLog, error)
	ListByUser(ctx context.Context, userID string, limit, offset int) ([]user.RoleChangeLog, error)
}

// AdminUserHandler exposes user-administration endpoints (role management,
// audit log lookup). Coin adjustments live in AdminCoinHandler.
type AdminUserHandler struct {
	userRepo       user.Repository
	roleChangeRepo RoleChangeRepoForHandler
}

func NewAdminUserHandler(userRepo user.Repository, roleChangeRepo RoleChangeRepoForHandler) *AdminUserHandler {
	return &AdminUserHandler{userRepo: userRepo, roleChangeRepo: roleChangeRepo}
}

func (h *AdminUserHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/search", h.SearchUser)
	group.POST("/role", h.UpdateRole)
	group.GET("/:id/role-history", h.RoleHistory)
}

// SearchUser handles GET /api/v1/admin/users/search?type=id|email&q=...
func (h *AdminUserHandler) SearchUser(c *gin.Context) {
	searchType := c.Query("type")
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "검색어를 입력해주세요"})
		return
	}

	var (
		u   user.User
		err error
	)
	switch searchType {
	case "id":
		u, err = h.userRepo.FindByID(c.Request.Context(), query)
	case "email":
		u, err = h.userRepo.FindByEmail(c.Request.Context(), query)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "검색 타입은 id 또는 email이어야 합니다"})
		return
	}

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "해당 유저를 찾을 수 없습니다"})
			return
		}
		slog.Error("admin user search error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "검색 중 오류가 발생했습니다"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": u})
}

type updateRoleRequest struct {
	UserID  string `json:"user_id" binding:"required"`
	NewRole string `json:"new_role" binding:"required"`
	Memo    string `json:"memo"`
}

// UpdateRole handles POST /api/v1/admin/users/role.
func (h *AdminUserHandler) UpdateRole(c *gin.Context) {
	adminID := c.GetString("userID")

	var req updateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id와 new_role은 필수입니다"})
		return
	}

	if !user.IsValidRole(req.NewRole) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "new_role 값은 user, editor, admin 중 하나여야 합니다"})
		return
	}

	if req.UserID == adminID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "본인의 권한은 변경할 수 없습니다"})
		return
	}

	target, err := h.userRepo.FindByID(c.Request.Context(), req.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "해당 유저를 찾을 수 없습니다"})
			return
		}
		slog.Error("admin update role find user error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "유저 조회 중 오류가 발생했습니다"})
		return
	}

	if target.Role == req.NewRole {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"user_id":      target.ID,
				"before_role":  target.Role,
				"after_role":   req.NewRole,
				"unchanged":    true,
			},
		})
		return
	}

	if err := h.userRepo.UpdateRole(c.Request.Context(), req.UserID, req.NewRole); err != nil {
		slog.Error("admin update role error", "admin_id", adminID, "target_id", req.UserID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "권한 변경 중 오류가 발생했습니다"})
		return
	}

	memo := strings.TrimSpace(req.Memo)
	actor := adminID
	logEntry, err := h.roleChangeRepo.Log(c.Request.Context(), user.RoleChangeLog{
		UserID:     req.UserID,
		BeforeRole: target.Role,
		AfterRole:  req.NewRole,
		ActorID:    &actor,
		Memo:       memo,
	})
	if err != nil {
		// Audit failure is logged but not surfaced — the role change itself succeeded.
		slog.Error("admin role change audit failed", "admin_id", adminID, "target_id", req.UserID, "error", err)
	}

	slog.Info("admin role change",
		"admin_id", adminID,
		"target_id", target.ID,
		"before_role", target.Role,
		"after_role", req.NewRole,
		"memo", memo,
	)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"user_id":     target.ID,
			"before_role": target.Role,
			"after_role":  req.NewRole,
			"log":         logEntry,
		},
	})
}

// RoleHistory handles GET /api/v1/admin/users/:id/role-history?limit=&offset=
func (h *AdminUserHandler) RoleHistory(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "유저 ID는 필수입니다"})
		return
	}

	limit, offset := parsePageParams(c, 50, 100)

	entries, err := h.roleChangeRepo.ListByUser(c.Request.Context(), userID, limit, offset)
	if err != nil {
		slog.Error("admin role history error", "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "이력 조회 중 오류가 발생했습니다"})
		return
	}

	if entries == nil {
		entries = []user.RoleChangeLog{}
	}
	c.JSON(http.StatusOK, gin.H{"data": entries})
}

