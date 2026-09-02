package main

import (
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

	// Init Services
	authSvc := service.NewAuthService(userRepo, cfg.JWTSecret)

	storage, err := service.NewLocalStorage(cfg.UploadDir)
	if err != nil {
		log.Fatalf("failed to init storage: %v", err)
	}
	resourceSvc := service.NewResourceService(resourceRepo, storage, userRepo, folderRepo)
	folderSvc := service.NewFolderService(folderRepo)
	trashSvc := service.NewTrashService(folderRepo, resourceRepo, userRepo, storage)

	// Init Handlers
	authHandler := handler.NewAuthHandler(authSvc)
	resourceHandler := handler.NewResourceHandler(resourceSvc)
	driveHandler := handler.NewDriveHandler(folderSvc, resourceSvc, trashSvc, authSvc)

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

	// Protected Routes (User Profile, etc.)
	api := r.PathPrefix("/api").Subrouter()
	api.Use(handler.AuthMiddleware(authSvc, cfg.JWTSecret))

	api.HandleFunc("/me", authHandler.Me).Methods("GET")
	api.HandleFunc("/me", authHandler.UpdateMe).Methods("PATCH")

	// Drive Routes (quota, folders, files, trash)
	api.HandleFunc("/drive/quota", driveHandler.GetQuota).Methods("GET")
	api.HandleFunc("/drive/items", driveHandler.ListItems).Methods("GET")
	api.HandleFunc("/drive/folders", driveHandler.ListFolders).Methods("GET")
	api.HandleFunc("/drive/folders", driveHandler.CreateFolder).Methods("POST")
	api.HandleFunc("/drive/folders/{id}", driveHandler.UpdateFolder).Methods("PATCH")
	api.HandleFunc("/drive/folders/{id}", driveHandler.DeleteFolder).Methods("DELETE")
	api.HandleFunc("/drive/files", driveHandler.UploadFile).Methods("POST")
	api.HandleFunc("/drive/files/{id}", driveHandler.UpdateFile).Methods("PATCH")
	api.HandleFunc("/drive/files/{id}", driveHandler.DeleteFile).Methods("DELETE")
	api.HandleFunc("/drive/files/{id}/download", driveHandler.DownloadFile).Methods("GET")
	api.HandleFunc("/drive/trash", driveHandler.ListTrash).Methods("GET")
	api.HandleFunc("/drive/trash/{kind}/{id}/restore", driveHandler.RestoreTrashItem).Methods("POST")
	api.HandleFunc("/drive/trash/{kind}/{id}", driveHandler.HardDeleteTrashItem).Methods("DELETE")

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
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Printf("Chirp server listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
