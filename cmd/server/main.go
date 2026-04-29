package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"allinonekey/internal/api"
	"allinonekey/internal/model"
	"allinonekey/internal/service"
	"allinonekey/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	dbPath := os.Getenv("ALLINONEKEY_DB_PATH")
	if dbPath == "" {
		dbPath = "data/allinone.db"
	}
	if dir := filepath.Dir(dbPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatal(err)
		}
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	db.AutoMigrate(&model.User{}, &model.APIKey{}, &model.Account{}, &model.InvitationCode{}, &model.AuditLog{})

	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	authH := &api.AuthHandler{DB: db}
	quotaService := &service.QuotaService{DB: db}
	keysH := &api.KeyHandler{DB: db, QuotaService: quotaService}
	accsH := &api.AccountHandler{DB: db}
	adminH := &api.AdminHandler{DB: db}
	auditH := &api.AuditHandler{DB: db}
	exportH := &api.ExportHandler{DB: db}

	go quotaService.StartCron()

	r.POST("/api/register", authH.Register)
	r.POST("/api/login", authH.Login)

	apiGroup := r.Group("/api")
	apiGroup.Use(AuthMiddleware())
	{
		apiGroup.GET("/keys/list", keysH.List)
		apiGroup.GET("/keys/stats", keysH.GetStats)
		apiGroup.POST("/keys/bulk", keysH.CreateBulk)
		apiGroup.POST("/keys/:id/check-quota", keysH.CheckQuota)
		apiGroup.PATCH("/keys/:id", keysH.Update)
		apiGroup.DELETE("/keys/:id", keysH.Delete)
		apiGroup.GET("/keys/:id/decrypt", keysH.Decrypt)

		apiGroup.GET("/accounts/list", accsH.List)
		apiGroup.POST("/accounts/create", accsH.Create)
		apiGroup.PATCH("/accounts/:id", accsH.Update)
		apiGroup.DELETE("/accounts/:id", accsH.Delete)
		apiGroup.GET("/accounts/:id/decrypt", accsH.Decrypt)
		apiGroup.GET("/accounts/:id/totp", accsH.TOTP)

		apiGroup.GET("/audit/list", auditH.List)

		apiGroup.GET("/export/json", exportH.ExportJSON)
		apiGroup.GET("/export/csv", exportH.ExportCSV)
		apiGroup.POST("/import/json", exportH.ImportJSON)

		adminGroup := apiGroup.Group("/admin")
		adminGroup.Use(AdminMiddleware())
		{
			adminGroup.GET("/invites", adminH.ListInvites)
			adminGroup.POST("/invites", adminH.CreateInvite)
			adminGroup.DELETE("/invites/:id", adminH.DeleteInvite)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server running on port %s", port)
	r.Run(":" + port)
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if tokenStr == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "Missing token"})
			return
		}
		session, err := util.OpenSession(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "Unauthorized"})
			return
		}
		c.Set("user_id", session.UserID)
		c.Set("user_role", session.Role)
		c.Set("master_key", session.MasterKey)
		c.Next()
	}
}

func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("user_role")
		if role != "admin" {
			c.AbortWithStatusJSON(403, gin.H{"error": "Forbidden"})
			return
		}
		c.Next()
	}
}
