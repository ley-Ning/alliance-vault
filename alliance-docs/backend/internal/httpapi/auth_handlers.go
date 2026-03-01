package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"alliance-vault/backend/internal/auth"
	"alliance-vault/backend/internal/model"
	"alliance-vault/backend/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type authUserResponse struct {
	ID                 string `json:"id"`
	Username           string `json:"username"`
	DisplayName        string `json:"displayName"`
	IsAdmin            bool   `json:"isAdmin"`
	IsDisabled         bool   `json:"isDisabled"`
	MustChangePassword bool   `json:"mustChangePassword"`
}

type authSessionResponse struct {
	User         authUserResponse `json:"user"`
	AccessToken  string           `json:"accessToken"`
	RefreshToken string           `json:"refreshToken"`
}

type createTeamMemberRequest struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"`
	DisplayName string `json:"displayName"`
}

func (s *Server) registerDisabled(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{"error": "已关闭自助注册，请联系管理员添加团队账号"})
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (s *Server) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "登录参数不完整", "detail": err.Error()})
		return
	}

	user, err := s.users.GetByUsername(c.Request.Context(), req.Username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户失败", "detail": err.Error()})
		return
	}
	if user.IsDisabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "账号已被禁用，请联系管理员"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	session, err := s.issueSession(c, user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "签发会话失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
}

func (s *Server) createTeamMember(c *gin.Context) {
	subject, ok := authSubjectFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "身份无效"})
		return
	}

	operator, err := s.users.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "校验管理员身份失败", "detail": err.Error()})
		return
	}
	if !operator.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可以添加团队成员"})
		return
	}

	var req createTeamMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新增成员参数不完整", "detail": err.Error()})
		return
	}

	username := strings.ToLower(strings.TrimSpace(req.Username))
	password := strings.TrimSpace(req.Password)
	displayName := strings.TrimSpace(req.DisplayName)

	if err := validateUsername(username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validatePassword(password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if displayName == "" {
		displayName = username
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败", "detail": err.Error()})
		return
	}

	user, err := s.users.Create(c.Request.Context(), model.User{
		ID:                 uuid.NewString(),
		Username:           username,
		DisplayName:        displayName,
		IsAdmin:            false,
		IsDisabled:         false,
		MustChangePassword: true,
		PasswordHash:       string(hashed),
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "新增成员失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"user": toAuthUser(user)})
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

func (s *Server) refreshToken(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "刷新参数不完整", "detail": err.Error()})
		return
	}

	claims, err := s.tokens.ParseToken(strings.TrimSpace(req.RefreshToken))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "刷新令牌无效", "detail": err.Error()})
		return
	}
	if claims.TokenType != auth.TokenTypeRefresh {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "刷新令牌类型错误"})
		return
	}
	if claims.ID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "刷新令牌缺少唯一标识"})
		return
	}

	active, err := s.refreshes.IsActive(c.Request.Context(), claims.ID, claims.Subject, time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "校验刷新令牌失败", "detail": err.Error()})
		return
	}
	if !active {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "刷新令牌已失效"})
		return
	}

	if err := s.refreshes.Revoke(c.Request.Context(), claims.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新刷新令牌失败", "detail": err.Error()})
		return
	}

	user, err := s.users.GetByID(c.Request.Context(), claims.Subject)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户失败", "detail": err.Error()})
		return
	}
	if user.IsDisabled {
		_ = s.refreshes.RevokeByUser(c.Request.Context(), user.ID)
		c.JSON(http.StatusForbidden, gin.H{"error": "账号已被禁用，请联系管理员"})
		return
	}

	session, err := s.issueSession(c, user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "签发会话失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
}

type logoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func (s *Server) logout(c *gin.Context) {
	var req logoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "退出参数不完整", "detail": err.Error()})
		return
	}

	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken != "" {
		claims, err := s.tokens.ParseToken(refreshToken)
		if err == nil && claims.TokenType == auth.TokenTypeRefresh && claims.ID != "" {
			_ = s.refreshes.Revoke(c.Request.Context(), claims.ID)
		}
	}

	c.JSON(http.StatusOK, gin.H{"loggedOut": true})
}

func (s *Server) me(c *gin.Context) {
	subject, ok := authSubjectFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "身份无效"})
		return
	}

	user, err := s.users.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": toAuthUser(user)})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword" binding:"required"`
}

func (s *Server) changePassword(c *gin.Context) {
	subject, ok := authSubjectFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "身份无效"})
		return
	}

	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "修改密码参数不完整", "detail": err.Error()})
		return
	}

	currentPassword := strings.TrimSpace(req.CurrentPassword)
	newPassword := strings.TrimSpace(req.NewPassword)

	if err := validatePassword(newPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := s.users.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户失败", "detail": err.Error()})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(newPassword)); err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码不能和旧密码一致"})
		return
	}

	if !user.MustChangePassword {
		if currentPassword == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "当前密码不能为空"})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "当前密码错误"})
			return
		}
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败", "detail": err.Error()})
		return
	}

	updated, err := s.users.UpdatePassword(c.Request.Context(), subject.UserID, string(hashed), false)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新密码失败", "detail": err.Error()})
		return
	}

	if err := s.refreshes.RevokeByUser(c.Request.Context(), subject.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新会话失败", "detail": err.Error()})
		return
	}

	session, err := s.issueSession(c, updated)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "签发会话失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
}

func (s *Server) issueSession(c *gin.Context, user model.User) (authSessionResponse, error) {
	pair, err := s.tokens.GenerateTokenPair(
		user.ID,
		user.Username,
		user.IsAdmin,
		user.MustChangePassword,
		time.Now().UTC(),
	)
	if err != nil {
		return authSessionResponse{}, err
	}

	if err := s.refreshes.Create(
		c.Request.Context(),
		uuid.NewString(),
		user.ID,
		pair.RefreshTokenID,
		pair.RefreshTokenExpiresAt,
	); err != nil {
		return authSessionResponse{}, err
	}

	return authSessionResponse{
		User:         toAuthUser(user),
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
	}, nil
}

func toAuthUser(user model.User) authUserResponse {
	return authUserResponse{
		ID:                 user.ID,
		Username:           user.Username,
		DisplayName:        user.DisplayName,
		IsAdmin:            user.IsAdmin,
		IsDisabled:         user.IsDisabled,
		MustChangePassword: user.MustChangePassword,
	}
}

func validateUsername(username string) error {
	if len(username) < 3 || len(username) > 32 {
		return errors.New("用户名长度需要在 3 到 32 之间")
	}
	for _, r := range username {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return errors.New("用户名仅支持小写字母、数字、下划线和短横线")
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("密码至少 8 位")
	}
	if len(password) > 64 {
		return errors.New("密码不能超过 64 位")
	}
	return nil
}
