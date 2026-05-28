package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ota/domain/comment"
)

// CommentHandler exposes the comment endpoints under /api/v1/comments.
//
// The handler is intentionally thin: validation lives in the domain layer,
// reaction logic lives in cache.ReactionStore, and pagination cursors are
// constructed by the storage layer. The handler's job is request shaping
// and response composition (folding in per-user reaction state and reply
// counts that the client expects on every list).
type CommentHandler struct {
	service     *comment.Service
	authMW      gin.HandlerFunc
	optAuthMW   gin.HandlerFunc
	adminCheck  gin.HandlerFunc // applied to admin-only routes
}

// NewCommentHandler wires the handler with the middlewares it will apply
// to individual routes.
func NewCommentHandler(service *comment.Service, authMW, optionalAuthMW, adminMW gin.HandlerFunc) *CommentHandler {
	return &CommentHandler{
		service:    service,
		authMW:     authMW,
		optAuthMW:  optionalAuthMW,
		adminCheck: adminMW,
	}
}

// RegisterRoutes implements api.RouteRegistrar. The "comments" group is
// expected; callers should pass a no-op middlewares slice in RouteModule.
func (h *CommentHandler) RegisterRoutes(group *gin.RouterGroup) {
	// Public listings — anonymous OK, but authenticated callers get
	// my_reaction in responses.
	group.GET("", h.optAuthMW, h.list)
	group.GET("/:id/replies", h.optAuthMW, h.replies)
	group.GET("/:id", h.optAuthMW, h.show)

	// Authenticated actions.
	group.POST("", h.authMW, h.create)
	group.PATCH("/:id", h.authMW, h.update)
	group.DELETE("/:id", h.authMW, h.delete)
	group.POST("/:id/react", h.authMW, h.react)
}

// --- Request DTOs ---

type createCommentReq struct {
	TargetType string  `json:"target_type" binding:"required"`
	TargetID   string  `json:"target_id" binding:"required"`
	ParentID   *string `json:"parent_id,omitempty"`
	Content    string  `json:"content" binding:"required"`
}

type updateCommentReq struct {
	Content string `json:"content" binding:"required"`
}

type reactReq struct {
	Reaction int `json:"reaction"` // 1=like, -1=dislike, 0=clear
}

// --- Response DTOs ---

type commentResponse struct {
	ID            string    `json:"id"`
	TargetType    string    `json:"target_type"`
	TargetID      string    `json:"target_id"`
	GroupID       string    `json:"group_id"`
	ParentID      *string   `json:"parent_id"`
	Depth         int       `json:"depth"`
	Content       string    `json:"content"`
	LikesCount    int       `json:"likes_count"`
	DislikesCount int       `json:"dislikes_count"`
	MyReaction    int       `json:"my_reaction"`
	ReplyCount    int       `json:"reply_count"`
	IsDeleted     bool      `json:"is_deleted"`
	CreatedAt     string    `json:"created_at"`
	EditedAt      *string   `json:"edited_at,omitempty"`
	Author        authorDTO `json:"author"`
}

type authorDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type listResponse struct {
	Items      []commentResponse `json:"items"`
	NextCursor string            `json:"next_cursor"`
}

// --- Handlers ---

func (h *CommentHandler) list(c *gin.Context) {
	target := comment.TargetType(c.Query("target_type"))
	if !target.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target_type"})
		return
	}
	targetID, err := uuid.Parse(c.Query("target_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target_id"})
		return
	}
	sort := comment.SortOrder(c.DefaultQuery("sort", string(comment.SortPopular)))
	cursor := c.Query("cursor")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	page, err := h.service.ListRoots(c.Request.Context(), target, targetID, sort, cursor, limit)
	if err != nil {
		writeServiceError(c, err, "list comments")
		return
	}

	resp, err := h.composeList(c.Request.Context(), page.Items, true /* includeReplyCount */, c.GetString("userID"))
	if err != nil {
		writeServiceError(c, err, "compose list")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": listResponse{Items: resp, NextCursor: page.NextCursor}})
}

func (h *CommentHandler) replies(c *gin.Context) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	cursor := c.Query("cursor")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	page, err := h.service.ListReplies(c.Request.Context(), groupID, cursor, limit)
	if err != nil {
		writeServiceError(c, err, "list replies")
		return
	}
	resp, err := h.composeList(c.Request.Context(), page.Items, false, c.GetString("userID"))
	if err != nil {
		writeServiceError(c, err, "compose replies")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": listResponse{Items: resp, NextCursor: page.NextCursor}})
}

func (h *CommentHandler) show(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	cm, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		writeServiceError(c, err, "get comment")
		return
	}
	resp, err := h.composeList(c.Request.Context(), []comment.Comment{cm}, true, c.GetString("userID"))
	if err != nil {
		writeServiceError(c, err, "compose comment")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": resp[0]})
}

func (h *CommentHandler) create(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("userID"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
		return
	}
	var req createCommentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	target := comment.TargetType(req.TargetType)
	targetID, err := uuid.Parse(req.TargetID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target_id"})
		return
	}
	var parentID *uuid.UUID
	if req.ParentID != nil && *req.ParentID != "" {
		p, err := uuid.Parse(*req.ParentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parent_id"})
			return
		}
		parentID = &p
	}

	cm, err := h.service.Create(c.Request.Context(), userID, target, targetID, parentID, req.Content)
	if err != nil {
		writeServiceError(c, err, "create comment")
		return
	}
	resp, err := h.composeList(c.Request.Context(), []comment.Comment{cm}, true, userID.String())
	if err != nil {
		writeServiceError(c, err, "compose created comment")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": resp[0]})
}

func (h *CommentHandler) update(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("userID"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req updateCommentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	cm, err := h.service.Update(c.Request.Context(), userID, id, req.Content)
	if err != nil {
		writeServiceError(c, err, "update comment")
		return
	}
	resp, err := h.composeList(c.Request.Context(), []comment.Comment{cm}, true, userID.String())
	if err != nil {
		writeServiceError(c, err, "compose updated comment")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": resp[0]})
}

func (h *CommentHandler) delete(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("userID"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	role := c.GetString("role")
	if role == "admin" {
		if err := h.service.AdminDelete(c.Request.Context(), id); err != nil {
			writeServiceError(c, err, "admin delete comment")
			return
		}
		c.Status(http.StatusNoContent)
		return
	}
	if err := h.service.Delete(c.Request.Context(), userID, id); err != nil {
		writeServiceError(c, err, "delete comment")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *CommentHandler) react(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("userID"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req reactReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	target := comment.Reaction(req.Reaction)
	res, err := h.service.React(c.Request.Context(), userID, id, target)
	if err != nil {
		writeServiceError(c, err, "react")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"likes_count":    res.Counts.Likes,
		"dislikes_count": res.Counts.Dislikes,
		"my_reaction":    int(res.Current),
	}})
}

// composeList enriches comments with live counts, per-user reactions, and
// (for roots) reply counts. It is the only place these batch lookups
// happen, so we keep the comment.Service free of presentation logic.
func (h *CommentHandler) composeList(ctx context.Context, items []comment.Comment, includeReplyCount bool, callerID string) ([]commentResponse, error) {
	if len(items) == 0 {
		return []commentResponse{}, nil
	}
	ids := make([]uuid.UUID, 0, len(items))
	for _, c := range items {
		ids = append(ids, c.ID)
	}
	counts, err := h.service.BatchCounts(ctx, ids)
	if err != nil {
		return nil, err
	}

	myReactions := map[uuid.UUID]comment.Reaction{}
	if callerID != "" {
		callerUUID, err := uuid.Parse(callerID)
		if err == nil {
			myReactions, err = h.service.BatchUserReactions(ctx, callerUUID, ids)
			if err != nil {
				return nil, err
			}
		}
	}

	replyCounts := map[uuid.UUID]int{}
	if includeReplyCount {
		groupIDs := make([]uuid.UUID, 0, len(items))
		for _, c := range items {
			if c.Depth == 0 {
				groupIDs = append(groupIDs, c.GroupID)
			}
		}
		if len(groupIDs) > 0 {
			rc, err := h.service.ReplyCounts(ctx, groupIDs)
			if err != nil {
				return nil, err
			}
			replyCounts = rc
		}
	}

	out := make([]commentResponse, 0, len(items))
	for _, c := range items {
		ct := counts[c.ID]
		// Live counts from Redis fall through to DB columns on cold miss.
		if ct.Likes == 0 && c.LikesCount > 0 {
			ct.Likes = c.LikesCount
		}
		if ct.Dislikes == 0 && c.DislikesCount > 0 {
			ct.Dislikes = c.DislikesCount
		}
		var parentStr *string
		if c.ParentID != nil {
			s := c.ParentID.String()
			parentStr = &s
		}
		var editedStr *string
		if c.EditedAt != nil {
			s := c.EditedAt.Format("2006-01-02T15:04:05.000Z07:00")
			editedStr = &s
		}
		resp := commentResponse{
			ID:            c.ID.String(),
			TargetType:    string(c.TargetType),
			TargetID:      c.TargetID.String(),
			GroupID:       c.GroupID.String(),
			ParentID:      parentStr,
			Depth:         c.Depth,
			Content:       c.Content,
			LikesCount:    ct.Likes,
			DislikesCount: ct.Dislikes,
			MyReaction:    int(myReactions[c.ID]),
			ReplyCount:    replyCounts[c.GroupID],
			IsDeleted:     c.IsDeleted(),
			CreatedAt:     c.CreatedAt.Format("2006-01-02T15:04:05.000Z07:00"),
			EditedAt:      editedStr,
			Author: authorDTO{
				ID:          c.UserID.String(),
				DisplayName: c.AuthorDisplayName(),
			},
		}
		if resp.IsDeleted {
			// Mask author for soft-deleted comments.
			resp.Author = authorDTO{}
			resp.Content = ""
		}
		out = append(out, resp)
	}
	return out, nil
}

// writeServiceError maps domain sentinel errors to HTTP status codes.
func writeServiceError(c *gin.Context, err error, op string) {
	switch {
	case errors.Is(err, comment.ErrAnonymousForbidden):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
	case errors.Is(err, comment.ErrCommentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "comment not found"})
	case errors.Is(err, comment.ErrNotOwner):
		c.JSON(http.StatusForbidden, gin.H{"error": "not the owner"})
	case errors.Is(err, comment.ErrAlreadyDeleted):
		c.JSON(http.StatusGone, gin.H{"error": "already deleted"})
	case errors.Is(err, comment.ErrInvalidContent):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid content"})
	case errors.Is(err, comment.ErrInvalidParent):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parent"})
	case errors.Is(err, comment.ErrInvalidTarget):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target"})
	case errors.Is(err, comment.ErrInvalidReaction):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reaction"})
	case errors.Is(err, comment.ErrTargetNotExist):
		c.JSON(http.StatusNotFound, gin.H{"error": "target not found"})
	default:
		slog.Error("comment handler error", "op", op, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
