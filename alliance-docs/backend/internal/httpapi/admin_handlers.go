package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"alliance-vault/backend/internal/model"
	"alliance-vault/backend/internal/store"

	"github.com/gin-gonic/gin"
)

type adminUserItem struct {
	ID                 string `json:"id"`
	Username           string `json:"username"`
	DisplayName        string `json:"displayName"`
	IsAdmin            bool   `json:"isAdmin"`
	IsDisabled         bool   `json:"isDisabled"`
	MustChangePassword bool   `json:"mustChangePassword"`
	DisabledAt         string `json:"disabledAt,omitempty"`
	CreatedAt          string `json:"createdAt"`
}

func toAdminUserItem(user model.User) adminUserItem {
	disabledAt := ""
	if user.DisabledAt != nil {
		disabledAt = user.DisabledAt.UTC().Format("2006-01-02T15:04:05Z")
	}

	return adminUserItem{
		ID:                 user.ID,
		Username:           user.Username,
		DisplayName:        user.DisplayName,
		IsAdmin:            user.IsAdmin,
		IsDisabled:         user.IsDisabled,
		MustChangePassword: user.MustChangePassword,
		DisabledAt:         disabledAt,
		CreatedAt:          user.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func (s *Server) listUsersForAdmin(c *gin.Context) {
	limit := 200
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	users, err := s.users.List(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询成员列表失败", "detail": err.Error()})
		return
	}

	items := make([]adminUserItem, 0, len(users))
	for _, user := range users {
		items = append(items, toAdminUserItem(user))
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

type updateUserRoleRequest struct {
	IsAdmin bool `json:"isAdmin"`
}

func (s *Server) updateUserRole(c *gin.Context) {
	subject, ok := authSubjectFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "身份无效"})
		return
	}

	userID := strings.TrimSpace(c.Param("id"))
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "成员 ID 不能为空"})
		return
	}

	var req updateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整", "detail": err.Error()})
		return
	}

	current, err := s.users.GetByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "成员不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询成员失败", "detail": err.Error()})
		return
	}

	if current.IsAdmin && !req.IsAdmin {
		if subject.UserID == userID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不能取消自己的管理员权限"})
			return
		}

		adminCount, countErr := s.users.CountAdmins(c.Request.Context())
		if countErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "校验管理员数量失败", "detail": countErr.Error()})
			return
		}
		if adminCount <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "系统至少需要保留一个管理员"})
			return
		}
	}

	updated, err := s.users.SetAdmin(c.Request.Context(), userID, req.IsAdmin)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "成员不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新成员角色失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": toAdminUserItem(updated)})
}

type updateUserDisabledRequest struct {
	IsDisabled bool `json:"isDisabled"`
}

func (s *Server) updateUserDisabled(c *gin.Context) {
	subject, ok := authSubjectFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "身份无效"})
		return
	}

	userID := strings.TrimSpace(c.Param("id"))
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "成员 ID 不能为空"})
		return
	}

	var req updateUserDisabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整", "detail": err.Error()})
		return
	}

	target, err := s.users.GetByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "成员不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询成员失败", "detail": err.Error()})
		return
	}

	if req.IsDisabled && subject.UserID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能禁用自己的账号"})
		return
	}

	if req.IsDisabled && target.IsAdmin {
		adminCount, countErr := s.users.CountAdmins(c.Request.Context())
		if countErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "校验管理员数量失败", "detail": countErr.Error()})
			return
		}
		if adminCount <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "系统至少需要保留一个可用管理员"})
			return
		}
	}

	updated, err := s.users.SetDisabled(c.Request.Context(), userID, req.IsDisabled)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "成员不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新账号状态失败", "detail": err.Error()})
		return
	}

	if req.IsDisabled {
		_ = s.refreshes.RevokeByUser(c.Request.Context(), userID)
	}

	c.JSON(http.StatusOK, gin.H{"user": toAdminUserItem(updated)})
}

func (s *Server) deleteUserForAdmin(c *gin.Context) {
	subject, ok := authSubjectFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "身份无效"})
		return
	}

	userID := strings.TrimSpace(c.Param("id"))
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "成员 ID 不能为空"})
		return
	}
	if userID == subject.UserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除自己的账号"})
		return
	}

	target, err := s.users.GetByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "成员不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询成员失败", "detail": err.Error()})
		return
	}

	if target.IsAdmin && !target.IsDisabled {
		adminCount, countErr := s.users.CountAdmins(c.Request.Context())
		if countErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "校验管理员数量失败", "detail": countErr.Error()})
			return
		}
		if adminCount <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "系统至少需要保留一个可用管理员"})
			return
		}
	}

	if err := s.users.DeleteByID(c.Request.Context(), userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "成员不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除账号失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true, "userId": userID})
}

type adminDocumentPermissionItem struct {
	DocumentID  string `json:"documentId"`
	UserID      string `json:"userId"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	IsAdmin     bool   `json:"isAdmin"`
	IsDisabled  bool   `json:"isDisabled"`
	AccessLevel string `json:"accessLevel"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

func toAdminDocumentPermissionItem(item store.DocumentPermissionDetail) adminDocumentPermissionItem {
	return adminDocumentPermissionItem{
		DocumentID:  item.DocumentID,
		UserID:      item.UserID,
		Username:    item.Username,
		DisplayName: item.DisplayName,
		IsAdmin:     item.IsAdmin,
		IsDisabled:  item.IsDisabled,
		AccessLevel: string(item.AccessLevel),
		CreatedAt:   item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func (s *Server) listDocumentPermissionsForAdmin(c *gin.Context) {
	documentID := strings.TrimSpace(c.Param("id"))
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文档 ID 不能为空"})
		return
	}

	if _, err := s.documents.GetByID(c.Request.Context(), documentID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询文档失败", "detail": err.Error()})
		return
	}

	items, err := s.documentPerms.ListByDocument(c.Request.Context(), documentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询文档权限失败", "detail": err.Error()})
		return
	}

	result := make([]adminDocumentPermissionItem, 0, len(items))
	for _, item := range items {
		result = append(result, toAdminDocumentPermissionItem(item))
	}

	c.JSON(http.StatusOK, gin.H{"items": result})
}

type upsertDocumentPermissionRequest struct {
	UserID      string `json:"userId" binding:"required"`
	AccessLevel string `json:"accessLevel" binding:"required"`
}

func (s *Server) upsertDocumentPermissionForAdmin(c *gin.Context) {
	documentID := strings.TrimSpace(c.Param("id"))
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文档 ID 不能为空"})
		return
	}

	var req upsertDocumentPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整", "detail": err.Error()})
		return
	}

	req.UserID = strings.TrimSpace(req.UserID)
	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "成员 ID 不能为空"})
		return
	}

	accessLevel := model.DocumentPermissionAccess(strings.TrimSpace(req.AccessLevel))
	if accessLevel != model.DocumentPermissionRead && accessLevel != model.DocumentPermissionEdit {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文档权限仅支持 read / edit"})
		return
	}

	if _, err := s.documents.GetByID(c.Request.Context(), documentID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询文档失败", "detail": err.Error()})
		return
	}
	if _, err := s.users.GetByID(c.Request.Context(), req.UserID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "成员不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询成员失败", "detail": err.Error()})
		return
	}

	item, err := s.documentPerms.Upsert(c.Request.Context(), documentID, req.UserID, accessLevel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文档权限失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"permission": toAdminDocumentPermissionItem(item)})
}

func (s *Server) deleteDocumentPermissionForAdmin(c *gin.Context) {
	documentID := strings.TrimSpace(c.Param("id"))
	userID := strings.TrimSpace(c.Param("userId"))
	if documentID == "" || userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文档 ID 与成员 ID 不能为空"})
		return
	}

	if err := s.documentPerms.Delete(c.Request.Context(), documentID, userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文档权限不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除文档权限失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
