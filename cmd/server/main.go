package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/zuquanzhi/Chirp/backend/internal/config"
	handler "github.com/zuquanzhi/Chirp/backend/internal/handler/http"
	"github.com/zuquanzhi/Chirp/backend/internal/repository/sqlite"
	"github.com/zuquanzhi/Chirp/backend/internal/service"
	"github.com/zuquanzhi/Chirp/backend/pkg/logger"
)

func main() {
	logFile, err := logger.Setup("logs")
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}
	defer logFile.Close()

	// Load Config
	cfg := config.Load()

	// Init Infrastructure (SQLite + Local FS)
	db, err := sqlite.InitDB(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer db.Close()

	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Fatalf("create uploads dir: %v", err)
	}

	// Init Repositories
	userRepo := sqlite.NewUserRepository(db)
	resourceRepo := sqlite.NewResourceRepository(db)
	folderRepo := sqlite.NewFolderRepository(db)
	shareRepo := sqlite.NewShareRepository(db)
	activityRepo := sqlite.NewActivityRepository(db)
	uploadSessionRepo := sqlite.NewUploadSessionRepository(db)

	// Init Services
	authSvc := service.NewAuthService(userRepo, cfg.JWTSecret)

	storage, err := service.NewLocalStorage(cfg.UploadDir)
	if err != nil {
		log.Fatalf("failed to init storage: %v", err)
	}
	resourceSvc := service.NewResourceService(resourceRepo, storage, userRepo, folderRepo)
	folderSvc := service.NewFolderService(folderRepo)
	trashSvc := service.NewTrashService(folderRepo, resourceRepo, userRepo, storage)
	shareSvc := service.NewShareService(shareRepo, resourceRepo, storage)
	uploadSvc, err := service.NewUploadService(uploadSessionRepo, resourceSvc, cfg.UploadDir)
	if err != nil {
		log.Fatalf("failed to init upload service: %v", err)
	}

	// Activity log recorder (best effort, shared by services)
	activityRec := service.NewActivityRecorder(activityRepo)
	resourceSvc.SetActivityRecorder(activityRec)
	folderSvc.SetActivityRecorder(activityRec)
	trashSvc.SetActivityRecorder(activityRec)
	shareSvc.SetActivityRecorder(activityRec)
	uploadSvc.SetActivityRecorder(activityRec)

	// Trash auto-cleanup: run at startup, then hourly (30-day retention).
	go func() {
		cleanup := func() {
			files, folders, err := trashSvc.CleanupExpired(context.Background(), service.TrashRetention)
			if err != nil {
				log.Printf("trash cleanup error: %v", err)
				return
			}
			if files+folders > 0 {
				log.Printf("trash cleanup: removed %d files, %d folders", files, folders)
			}
		}
		cleanup()
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			cleanup()
		}
	}()

	// Init Handlers
	authHandler := handler.NewAuthHandler(authSvc)
	resourceHandler := handler.NewResourceHandler(resourceSvc)
	driveHandler := handler.NewDriveHandler(folderSvc, resourceSvc, trashSvc, authSvc, shareSvc, uploadSvc, activityRec)

	// Setup Router
	r := mux.NewRouter()
	r.Use(handler.RecoverMiddleware)
	r.Use(handler.LoggingMiddleware)

	// Public Routes
	r.HandleFunc("/signup", authHandler.Signup).Methods("POST")
	r.HandleFunc("/login", authHandler.Login).Methods("POST")

	publicRes := r.PathPrefix("/api/public").Subrouter()
	// Use OptionalAuthMiddleware to attach user info if token is present
	publicRes.Use(handler.OptionalAuthMiddleware(authSvc, cfg.JWTSecret))
	publicRes.HandleFunc("/resources", resourceHandler.Upload).Methods("POST")
	publicRes.HandleFunc("/resources", resourceHandler.List).Methods("GET")
	publicRes.HandleFunc("/resources/{id}/download", resourceHandler.Download).Methods("GET")

	// Public share access (no auth)
	r.HandleFunc("/api/shares/{token}", driveHandler.GetShareInfo).Methods("GET")
	r.HandleFunc("/api/shares/{token}/download", driveHandler.DownloadShare).Methods("GET")

	// Protected Routes (User Profile, etc.)
	api := r.PathPrefix("/api").Subrouter()
	api.Use(handler.AuthMiddleware(authSvc, cfg.JWTSecret))

	api.HandleFunc("/me", authHandler.Me).Methods("GET")
	api.HandleFunc("/me", authHandler.UpdateMe).Methods("PATCH")
	api.HandleFunc("/activities", driveHandler.ListActivities).Methods("GET")

	// Drive Routes (quota, folders, files, trash)
	api.HandleFunc("/drive/quota", driveHandler.GetQuota).Methods("GET")
	api.HandleFunc("/drive/items", driveHandler.ListItems).Methods("GET")
	api.HandleFunc("/drive/folders", driveHandler.ListFolders).Methods("GET")
	api.HandleFunc("/drive/folders", driveHandler.CreateFolder).Methods("POST")
	api.HandleFunc("/drive/folders/{id}", driveHandler.UpdateFolder).Methods("PATCH")
	api.HandleFunc("/drive/folders/{id}", driveHandler.DeleteFolder).Methods("DELETE")
	api.HandleFunc("/drive/files", driveHandler.UploadFile).Methods("POST")
	api.HandleFunc("/drive/files/instant", driveHandler.InstantUpload).Methods("POST")
	api.HandleFunc("/drive/files/{id}", driveHandler.UpdateFile).Methods("PATCH")
	api.HandleFunc("/drive/files/{id}", driveHandler.DeleteFile).Methods("DELETE")
	api.HandleFunc("/drive/files/{id}/download", driveHandler.DownloadFile).Methods("GET")
	api.HandleFunc("/drive/files/{id}/versions", driveHandler.ListFileVersions).Methods("GET")
	api.HandleFunc("/drive/files/{id}/versions/{vid}/restore", driveHandler.RestoreFileVersion).Methods("POST")
	api.HandleFunc("/drive/trash", driveHandler.ListTrash).Methods("GET")
	api.HandleFunc("/drive/trash/{kind}/{id}/restore", driveHandler.RestoreTrashItem).Methods("POST")
	api.HandleFunc("/drive/trash/{kind}/{id}", driveHandler.HardDeleteTrashItem).Methods("DELETE")

	// Batch operations
	api.HandleFunc("/drive/batch/delete", driveHandler.BatchDelete).Methods("POST")
	api.HandleFunc("/drive/batch/move", driveHandler.BatchMove).Methods("POST")
	api.HandleFunc("/drive/batch/download", driveHandler.BatchDownload).Methods("POST")

	// Share management
	api.HandleFunc("/drive/shares", driveHandler.CreateShare).Methods("POST")
	api.HandleFunc("/drive/shares", driveHandler.ListShares).Methods("GET")
	api.HandleFunc("/drive/shares/{id}", driveHandler.DeleteShare).Methods("DELETE")

	// Chunked (resumable) uploads
	api.HandleFunc("/drive/uploads/init", driveHandler.InitChunkedUpload).Methods("POST")
	api.HandleFunc("/drive/uploads/{id}", driveHandler.GetChunkedUpload).Methods("GET")
	api.HandleFunc("/drive/uploads/{id}", driveHandler.AbortChunkedUpload).Methods("DELETE")
	api.HandleFunc("/drive/uploads/{id}/chunks/{index}", driveHandler.UploadChunk).Methods("PUT")
	api.HandleFunc("/drive/uploads/{id}/complete", driveHandler.CompleteChunkedUpload).Methods("POST")

	// Admin Routes (Review, etc.)
	admin := r.PathPrefix("/api/admin").Subrouter()
	admin.Use(handler.AuthMiddleware(authSvc, cfg.JWTSecret))
	admin.Use(handler.AdminMiddleware)
	admin.HandleFunc("/resources/{id}/review", resourceHandler.Review).Methods("POST")
	admin.HandleFunc("/resources/duplicates", resourceHandler.CheckDuplicate).Methods("GET")

	// Static files (optional, usually handled by Nginx)
	r.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadDir))))

	// Start Server
	srv := &http.Server{
		Handler:      r,
		Addr:         ":" + cfg.Port,
		WriteTimeout: 15 * time.Minute, // large uploads / zip downloads need more than 15s
		ReadTimeout:  15 * time.Minute,
	}

	log.Printf("Chirp server listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
