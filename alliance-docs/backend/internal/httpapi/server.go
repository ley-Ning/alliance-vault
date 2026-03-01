package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"alliance-vault/backend/internal/auth"
	"alliance-vault/backend/internal/config"
	"alliance-vault/backend/internal/model"
	"alliance-vault/backend/internal/storage"
	"alliance-vault/backend/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Server struct {
	cfg              config.Config
	attachments      *store.AttachmentRepo
	documents        *store.DocumentRepo
	documentVersions *store.DocumentVersionRepo
	documentPerms    *store.DocumentPermissionRepo
	users            *store.UserRepo
	refreshes        *store.RefreshTokenRepo
	storage          *storage.MinioClient
	tokens           *auth.TokenManager
}

func NewServer(
	cfg config.Config,
	attachmentRepo *store.AttachmentRepo,
	documentRepo *store.DocumentRepo,
	documentVersionRepo *store.DocumentVersionRepo,
	documentPermissionRepo *store.DocumentPermissionRepo,
	userRepo *store.UserRepo,
	refreshRepo *store.RefreshTokenRepo,
	objectStorageClient *storage.MinioClient,
	tokenManager *auth.TokenManager,
) *Server {
	return &Server{
		cfg:              cfg,
		attachments:      attachmentRepo,
		documents:        documentRepo,
		documentVersions: documentVersionRepo,
		documentPerms:    documentPermissionRepo,
		users:            userRepo,
		refreshes:        refreshRepo,
		storage:          objectStorageClient,
		tokens:           tokenManager,
	}
}

func (s *Server) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(s.corsMiddleware())

	api := r.Group("/api/v1")
	api.GET("/health", s.health)

	api.POST("/auth/register", s.registerDisabled)
	api.POST("/auth/login", s.login)
	api.POST("/auth/refresh", s.refreshToken)

	protected := api.Group("")
	protected.Use(s.authMiddleware())
	protected.GET("/auth/me", s.me)
	protected.POST("/auth/logout", s.logout)
	protected.POST("/auth/change-password", s.changePassword)

	ready := protected.Group("")
	ready.Use(s.requirePasswordChanged())
	ready.POST("/auth/team-members", s.createTeamMember)
	admin := ready.Group("/admin")
	admin.Use(s.requireAdmin())
	admin.GET("/users", s.listUsersForAdmin)
	admin.PATCH("/users/:id/role", s.updateUserRole)
	admin.PATCH("/users/:id/disabled", s.updateUserDisabled)
	admin.DELETE("/users/:id", s.deleteUserForAdmin)
	admin.GET("/documents/:id/permissions", s.listDocumentPermissionsForAdmin)
	admin.PUT("/documents/:id/permissions", s.upsertDocumentPermissionForAdmin)
	admin.DELETE("/documents/:id/permissions/:userId", s.deleteDocumentPermissionForAdmin)

	ready.GET("/documents", s.listDocuments)
	ready.GET("/documents/recycle-bin", s.listDeletedDocuments)
	ready.POST("/documents", s.createDocument)
	ready.GET("/documents/:id/attachments", s.listAttachments)
	ready.GET("/documents/:id", s.getDocument)
	ready.GET("/documents/:id/versions", s.listDocumentVersions)
	ready.POST("/documents/:id/versions/:versionId/rollback", s.rollbackDocumentVersion)
	ready.PATCH("/documents/:id", s.updateDocument)
	ready.DELETE("/documents/:id", s.deleteDocument)
	ready.POST("/documents/:id/restore", s.restoreDocument)

	ready.POST("/uploads/presign", s.presignUpload)
	ready.POST("/uploads/complete", s.completeUpload)
	ready.GET("/attachments/:id", s.getAttachment)
	ready.GET("/attachments/:id/download-url", s.getDownloadURL)
	ready.DELETE("/attachments/:id", s.deleteAttachment)

	return r
}

func (s *Server) corsMiddleware() gin.HandlerFunc {
	allowedOrigins := parseAllowedOrigins(s.cfg.FrontendOrigin)
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{
			"http://localhost:8080",
			"http://127.0.0.1:8080",
			"http://localhost:5173",
			"http://127.0.0.1:5173",
			"null",
		}
	}

	wildcardAllowed := false
	for _, item := range allowedOrigins {
		if item == "*" {
			wildcardAllowed = true
			break
		}
	}

	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if wildcardAllowed {
			c.Header("Access-Control-Allow-Origin", "*")
		} else if origin != "" {
			for _, allowedOrigin := range allowedOrigins {
				if origin == allowedOrigin {
					c.Header("Access-Control-Allow-Origin", origin)
					c.Header("Vary", "Origin")
					break
				}
			}
		}

		c.Header("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func parseAllowedOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		origins = append(origins, origin)
	}
	return origins
}

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

type listDocumentsResponse struct {
	Items []model.Document `json:"items"`
}

func (s *Server) listDocuments(c *gin.Context) {
	subject, ok := authSubjectFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "身份无效"})
		return
	}

	limit := 100
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	items, err := s.documents.List(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询文档列表失败", "detail": err.Error()})
		return
	}

	if subject.IsAdmin {
		for index := range items {
			items[index].CanEdit = true
		}
		c.JSON(http.StatusOK, listDocumentsResponse{Items: items})
		return
	}

	filtered := make([]model.Document, 0, len(items))
	for _, item := range items {
		canRead, canEdit, accessErr := s.documentPerms.GetAccess(c.Request.Context(), item.ID, subject.UserID, false)
		if accessErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "校验文档权限失败", "detail": accessErr.Error()})
			return
		}
		if !canRead {
			continue
		}
		item.CanEdit = canEdit
		filtered = append(filtered, item)
	}

	c.JSON(http.StatusOK, listDocumentsResponse{Items: filtered})
}

func (s *Server) listDeletedDocuments(c *gin.Context) {
	subject, ok := authSubjectFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "身份无效"})
		return
	}

	limit := 100
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	items, err := s.documents.ListDeleted(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询回收站失败", "detail": err.Error()})
		return
	}

	if subject.IsAdmin {
		for index := range items {
			items[index].CanEdit = true
		}
		c.JSON(http.StatusOK, listDocumentsResponse{Items: items})
		return
	}

	filtered := make([]model.Document, 0, len(items))
	for _, item := range items {
		canRead, canEdit, accessErr := s.documentPerms.GetAccess(c.Request.Context(), item.ID, subject.UserID, false)
		if accessErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "校验文档权限失败", "detail": accessErr.Error()})
			return
		}
		if !canRead {
			continue
		}
		item.CanEdit = canEdit
		filtered = append(filtered, item)
	}

	c.JSON(http.StatusOK, listDocumentsResponse{Items: filtered})
}

type createDocumentRequest struct {
	Title   string               `json:"title"`
	Content string               `json:"content"`
	Tags    []string             `json:"tags"`
	Status  model.DocumentStatus `json:"status"`
	Owner   string               `json:"owner"`
}

func (s *Server) createDocument(c *gin.Context) {
	var req createDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整", "detail": err.Error()})
		return
	}

	if req.Status != "" && !isValidStatus(req.Status) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文档状态不合法"})
		return
	}

	subject, ok := authSubjectFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "身份无效，请重新登录"})
		return
	}

	owner := strings.TrimSpace(req.Owner)
	if owner == "" {
		owner = subject.Username
	}

	doc, err := s.documents.Create(c.Request.Context(), model.Document{
		ID:      uuid.NewString(),
		Title:   req.Title,
		Content: req.Content,
		Tags:    req.Tags,
		Status:  req.Status,
		Owner:   owner,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建文档失败", "detail": err.Error()})
		return
	}
	_ = s.captureVersion(c, doc, model.DocumentVersionEventCreate)
	doc.CanEdit = true

	c.JSON(http.StatusCreated, gin.H{"document": doc})
}

func (s *Server) getDocument(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	subject, ok := authSubjectFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "身份无效"})
		return
	}
	doc, err := s.documents.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询文档失败", "detail": err.Error()})
		return
	}
	canRead, canEdit, accessErr := s.documentPerms.GetAccess(c.Request.Context(), id, subject.UserID, subject.IsAdmin)
	if accessErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "校验文档权限失败", "detail": accessErr.Error()})
		return
	}
	if !canRead {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该文档"})
		return
	}
	doc.CanEdit = canEdit

	c.JSON(http.StatusOK, gin.H{"document": doc})
}

type updateDocumentRequest struct {
	Title   *string               `json:"title"`
	Content *string               `json:"content"`
	Tags    *[]string             `json:"tags"`
	Status  *model.DocumentStatus `json:"status"`
	Owner   *string               `json:"owner"`
}

func (s *Server) updateDocument(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if ok := s.ensureDocumentAccess(c, id, true); !ok {
		return
	}
	var req updateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整", "detail": err.Error()})
		return
	}

	if req.Status != nil && !isValidStatus(*req.Status) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文档状态不合法"})
		return
	}

	current, err := s.documents.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文档失败", "detail": err.Error()})
		return
	}
	if err := s.captureVersion(c, current, model.DocumentVersionEventUpdate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "记录历史版本失败", "detail": err.Error()})
		return
	}

	updated, err := s.documents.Update(c.Request.Context(), id, store.DocumentPatch{
		Title:   req.Title,
		Content: req.Content,
		Tags:    req.Tags,
		Status:  req.Status,
		Owner:   req.Owner,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新文档失败", "detail": err.Error()})
		return
	}
	updated.CanEdit = true

	c.JSON(http.StatusOK, gin.H{"document": updated})
}

func (s *Server) deleteDocument(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文档 ID 不能为空"})
		return
	}
	if ok := s.ensureDocumentAccess(c, id, true); !ok {
		return
	}

	current, err := s.documents.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文档失败", "detail": err.Error()})
		return
	}
	if err := s.captureVersion(c, current, model.DocumentVersionEventDelete); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "记录删除快照失败", "detail": err.Error()})
		return
	}

	if err := s.documents.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除文档失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (s *Server) restoreDocument(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文档 ID 不能为空"})
		return
	}
	if ok := s.ensureDocumentAccess(c, id, true); !ok {
		return
	}

	restored, err := s.documents.Restore(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "回收站中不存在该文档"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "恢复文档失败", "detail": err.Error()})
		return
	}
	if err := s.captureVersion(c, restored, model.DocumentVersionEventRestore); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "记录恢复快照失败", "detail": err.Error()})
		return
	}
	restored.CanEdit = true

	c.JSON(http.StatusOK, gin.H{"document": restored})
}

type listDocumentVersionsResponse struct {
	Items []model.DocumentVersion `json:"items"`
}

func (s *Server) listDocumentVersions(c *gin.Context) {
	documentID := strings.TrimSpace(c.Param("id"))
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文档 ID 不能为空"})
		return
	}
	if ok := s.ensureDocumentAccess(c, documentID, false); !ok {
		return
	}

	if _, err := s.documents.GetByID(c.Request.Context(), documentID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文档失败", "detail": err.Error()})
		return
	}

	limit := 50
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	items, err := s.documentVersions.ListByDocument(c.Request.Context(), documentID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询历史版本失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, listDocumentVersionsResponse{Items: items})
}

func (s *Server) rollbackDocumentVersion(c *gin.Context) {
	documentID := strings.TrimSpace(c.Param("id"))
	versionID := strings.TrimSpace(c.Param("versionId"))
	if documentID == "" || versionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}
	if ok := s.ensureDocumentAccess(c, documentID, true); !ok {
		return
	}

	current, err := s.documents.GetByID(c.Request.Context(), documentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文档失败", "detail": err.Error()})
		return
	}
	target, err := s.documentVersions.GetByID(c.Request.Context(), documentID, versionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "历史版本不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取历史版本失败", "detail": err.Error()})
		return
	}
	if err := s.captureVersion(c, current, model.DocumentVersionEventRollbackBackup); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "回滚前备份失败", "detail": err.Error()})
		return
	}

	updated, err := s.documents.Update(c.Request.Context(), documentID, store.DocumentPatch{
		Title:   &target.Title,
		Content: &target.Content,
		Tags:    &target.Tags,
		Status:  &target.Status,
		Owner:   &target.Owner,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "回滚历史版本失败", "detail": err.Error()})
		return
	}
	updated.CanEdit = true

	c.JSON(http.StatusOK, gin.H{
		"document":         updated,
		"rolledBackFromId": target.ID,
		"rolledBackToVer":  target.Version,
	})
}

type presignUploadRequest struct {
	DocumentID  string `json:"documentId" binding:"required"`
	FileName    string `json:"fileName" binding:"required"`
	ContentType string `json:"contentType" binding:"required"`
	SizeBytes   int64  `json:"sizeBytes" binding:"required"`
}

func (s *Server) presignUpload(c *gin.Context) {
	var req presignUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整", "detail": err.Error()})
		return
	}

	req.DocumentID = strings.TrimSpace(req.DocumentID)
	req.FileName = strings.TrimSpace(req.FileName)
	req.ContentType = strings.TrimSpace(req.ContentType)

	if req.SizeBytes <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件大小必须大于 0"})
		return
	}
	if req.SizeBytes > s.cfg.MaxUploadSizeMB*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件超过大小限制"})
		return
	}

	if _, err := s.documents.GetByID(c.Request.Context(), req.DocumentID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在，无法上传附件"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "校验文档失败", "detail": err.Error()})
		return
	}
	if ok := s.ensureDocumentAccess(c, req.DocumentID, true); !ok {
		return
	}

	randomID := uuid.NewString()
	objectKey := storage.BuildObjectKey(req.DocumentID, req.FileName, randomID)

	uploadURL, err := s.storage.PresignUploadURL(c.Request.Context(), objectKey, s.cfg.UploadURLExpire)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成上传地址失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"objectKey":          objectKey,
		"uploadUrl":          uploadURL,
		"method":             "PUT",
		"requiredHeaders":    gin.H{"Content-Type": req.ContentType},
		"expiresInSeconds":   int64(s.cfg.UploadURLExpire.Seconds()),
		"maxUploadSizeBytes": s.cfg.MaxUploadSizeMB * 1024 * 1024,
	})
}

type completeUploadRequest struct {
	DocumentID  string `json:"documentId" binding:"required"`
	ObjectKey   string `json:"objectKey" binding:"required"`
	FileName    string `json:"fileName" binding:"required"`
	ContentType string `json:"contentType" binding:"required"`
	SizeBytes   int64  `json:"sizeBytes" binding:"required"`
	Owner       string `json:"owner"`
}

func (s *Server) completeUpload(c *gin.Context) {
	var req completeUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整", "detail": err.Error()})
		return
	}

	req.DocumentID = strings.TrimSpace(req.DocumentID)
	req.ObjectKey = strings.TrimSpace(req.ObjectKey)
	req.FileName = strings.TrimSpace(req.FileName)
	req.ContentType = strings.TrimSpace(req.ContentType)
	req.Owner = strings.TrimSpace(req.Owner)
	subject, ok := authSubjectFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "身份无效，请重新登录"})
		return
	}
	if req.Owner == "" {
		req.Owner = subject.Username
	}

	if _, err := s.documents.GetByID(c.Request.Context(), req.DocumentID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在，无法写入附件"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "校验文档失败", "detail": err.Error()})
		return
	}
	if ok := s.ensureDocumentAccess(c, req.DocumentID, true); !ok {
		return
	}

	exists, actualSize, err := s.storage.ObjectExists(c.Request.Context(), req.ObjectKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "校验上传文件失败", "detail": err.Error()})
		return
	}
	if !exists {
		c.JSON(http.StatusConflict, gin.H{"error": "文件尚未上传成功"})
		return
	}

	if req.SizeBytes > 0 && req.SizeBytes != actualSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件大小不一致"})
		return
	}

	attachment, err := s.attachments.Create(c.Request.Context(), model.Attachment{
		ID:          uuid.NewString(),
		DocumentID:  req.DocumentID,
		ObjectKey:   req.ObjectKey,
		FileName:    req.FileName,
		ContentType: req.ContentType,
		SizeBytes:   actualSize,
		Owner:       req.Owner,
		Storage:     "rustfs",
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存附件记录失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"attachment": attachment})
}

func (s *Server) getAttachment(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	attachment, err := s.attachments.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "附件不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询附件失败", "detail": err.Error()})
		return
	}
	if ok := s.ensureDocumentAccess(c, attachment.DocumentID, false); !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"attachment": attachment})
}

func (s *Server) listAttachments(c *gin.Context) {
	documentID := strings.TrimSpace(c.Param("id"))
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "documentId 不能为空"})
		return
	}

	if _, err := s.documents.GetByID(c.Request.Context(), documentID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "校验文档失败", "detail": err.Error()})
		return
	}
	if ok := s.ensureDocumentAccess(c, documentID, false); !ok {
		return
	}

	limit := 50
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	attachments, err := s.attachments.ListByDocument(c.Request.Context(), documentID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询附件列表失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": attachments})
}

func (s *Server) getDownloadURL(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	attachment, err := s.attachments.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "附件不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询附件失败", "detail": err.Error()})
		return
	}
	if ok := s.ensureDocumentAccess(c, attachment.DocumentID, false); !ok {
		return
	}

	downloadURL, err := s.storage.PresignDownloadURL(c.Request.Context(), attachment.ObjectKey, s.cfg.DownloadURLExpire)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成下载链接失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"downloadUrl":      downloadURL,
		"expiresInSeconds": int64(s.cfg.DownloadURLExpire.Seconds()),
		"attachment":       attachment,
	})
}

func (s *Server) deleteAttachment(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "附件 ID 不能为空"})
		return
	}

	attachment, err := s.attachments.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "附件不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询附件失败", "detail": err.Error()})
		return
	}
	if ok := s.ensureDocumentAccess(c, attachment.DocumentID, true); !ok {
		return
	}

	if err := s.storage.RemoveObject(c.Request.Context(), attachment.ObjectKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除对象存储文件失败", "detail": err.Error()})
		return
	}

	if _, err := s.attachments.DeleteByID(c.Request.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "附件不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除附件记录失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true, "attachmentId": id})
}

func isValidStatus(status model.DocumentStatus) bool {
	switch status {
	case model.DocumentStatusDraft, model.DocumentStatusReviewing, model.DocumentStatusPublished:
		return true
	default:
		return false
	}
}

func (s *Server) ensureDocumentAccess(c *gin.Context, documentID string, requireEdit bool) bool {
	subject, ok := authSubjectFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "身份无效"})
		return false
	}

	canRead, canEdit, err := s.documentPerms.GetAccess(c.Request.Context(), documentID, subject.UserID, subject.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "校验文档权限失败", "detail": err.Error()})
		return false
	}

	if requireEdit {
		if !canEdit {
			c.JSON(http.StatusForbidden, gin.H{"error": "当前仅有只读权限，无法修改文档"})
			return false
		}
		return true
	}

	if !canRead {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该文档"})
		return false
	}

	return true
}

func (s *Server) captureVersion(c *gin.Context, doc model.Document, event model.DocumentVersionEvent) error {
	if s.documentVersions == nil {
		return nil
	}

	createdBy := "system"
	if subject, ok := authSubjectFromContext(c); ok {
		createdBy = subject.Username
	}

	_, err := s.documentVersions.CreateSnapshot(c.Request.Context(), doc, event, createdBy)
	return err
}

type authSubject struct {
	UserID             string
	Username           string
	IsAdmin            bool
	IsDisabled         bool
	MustChangePassword bool
}

func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawAuth := strings.TrimSpace(c.GetHeader("Authorization"))
		if rawAuth == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "缺少登录令牌"})
			return
		}

		parts := strings.SplitN(rawAuth, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "登录令牌格式错误"})
			return
		}

		claims, err := s.tokens.ParseToken(strings.TrimSpace(parts[1]))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "登录令牌无效", "detail": err.Error()})
			return
		}
		if claims.TokenType != auth.TokenTypeAccess {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "访问令牌类型不正确"})
			return
		}

		user, err := s.users.GetByID(c.Request.Context(), claims.Subject)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "校验用户状态失败", "detail": err.Error()})
			return
		}
		if user.IsDisabled {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "账号已被禁用，请联系管理员"})
			return
		}

		c.Set("authSubject", authSubject{
			UserID:             user.ID,
			Username:           user.Username,
			IsAdmin:            user.IsAdmin,
			IsDisabled:         user.IsDisabled,
			MustChangePassword: user.MustChangePassword,
		})
		c.Next()
	}
}

func authSubjectFromContext(c *gin.Context) (authSubject, bool) {
	value, exists := c.Get("authSubject")
	if !exists {
		return authSubject{}, false
	}
	subject, ok := value.(authSubject)
	if !ok {
		return authSubject{}, false
	}
	if subject.UserID == "" || subject.Username == "" {
		return authSubject{}, false
	}
	return subject, true
}

func (s *Server) requirePasswordChanged() gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := authSubjectFromContext(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "身份无效"})
			return
		}
		if subject.MustChangePassword {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "首次登录请先修改密码"})
			return
		}
		c.Next()
	}
}

func (s *Server) requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := authSubjectFromContext(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "身份无效"})
			return
		}

		user, err := s.users.GetByID(c.Request.Context(), subject.UserID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "校验管理员身份失败", "detail": err.Error()})
			return
		}
		if !user.IsAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "仅管理员可访问该功能"})
			return
		}
		c.Next()
	}
}

func bearerTokenFromHeader(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	parts := strings.SplitN(raw, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", fmt.Errorf("invalid authorization header")
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", fmt.Errorf("token is empty")
	}
	return token, nil
}
