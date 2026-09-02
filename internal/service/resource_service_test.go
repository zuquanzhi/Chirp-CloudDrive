package service

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"testing"

	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

// fakeMultipartFile adapts a bytes.Reader to multipart.File.
type fakeMultipartFile struct {
	*bytes.Reader
}

func (f fakeMultipartFile) Close() error { return nil }

func newUpload(content []byte, name string) (multipart.File, *multipart.FileHeader) {
	return fakeMultipartFile{bytes.NewReader(content)},
		&multipart.FileHeader{Filename: name, Size: int64(len(content))}
}

type resourceFixture struct {
	svc     *ResourceService
	resRepo *fakeResourceRepo
	folders *fakeFolderRepo
	users   *fakeUserRepo
	storage *fakeStorage
}

func newResourceSvc() *resourceFixture {
	resRepo := newFakeResourceRepo()
	folders := newFakeFolderRepo()
	users := newFakeUserRepo()
	storage := newFakeStorage()
	return &resourceFixture{
		svc:     NewResourceService(resRepo, storage, users, folders),
		resRepo: resRepo,
		folders: folders,
		users:   users,
		storage: storage,
	}
}

// 上传成功：写存储、记账、状态 APPROVED、归属文件夹
func TestUploadToFolder_Success(t *testing.T) {
	fx := newResourceSvc()
	ctx := context.Background()
	fx.users.add(&domain.User{ID: 1, Quota: 1000, Used: 100})
	folderSvc := NewFolderService(fx.folders)
	folder, _ := folderSvc.Create(ctx, 1, "docs", nil)

	content := []byte("hello chirp")
	file, header := newUpload(content, "note.txt")
	res, err := fx.svc.UploadToFolder(ctx, 1, &folder.ID, file, header)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	if res.FolderID == nil || *res.FolderID != folder.ID {
		t.Fatal("file should belong to the folder")
	}
	if res.Status != domain.ResourceStatusApproved {
		t.Fatalf("drive file should be APPROVED, got %s", res.Status)
	}
	if res.URL == "" {
		t.Fatal("URL should be populated")
	}

	// 配额记账
	u, _ := fx.users.GetByID(ctx, 1)
	want := int64(100 + len(content))
	if u.Used != want {
		t.Fatalf("used = %d, want %d", u.Used, want)
	}

	// 物理文件真的写进去了
	got, err := fx.storage.Get(ctx, res.Filename)
	if err != nil {
		t.Fatalf("storage.Get: %v", err)
	}
	defer got.Close()
	data, _ := io.ReadAll(got)
	if !bytes.Equal(data, content) {
		t.Fatal("stored content mismatch")
	}
}

// 配额超限：拒绝上传，且不能写存储、不能记账
func TestUploadToFolder_QuotaExceeded(t *testing.T) {
	fx := newResourceSvc()
	ctx := context.Background()
	fx.users.add(&domain.User{ID: 1, Quota: 100, Used: 90})

	content := make([]byte, 20) // 90 + 20 > 100
	file, header := newUpload(content, "big.bin")
	if _, err := fx.svc.UploadToFolder(ctx, 1, nil, file, header); err == nil {
		t.Fatal("expected quota exceeded error")
	}

	u, _ := fx.users.GetByID(ctx, 1)
	if u.Used != 90 {
		t.Fatalf("used should stay 90, got %d", u.Used)
	}
	if len(fx.storage.files) != 0 {
		t.Fatal("nothing should be written to storage on quota failure")
	}
	if len(fx.resRepo.resources) != 0 {
		t.Fatal("no resource row should be created on quota failure")
	}
}

// 恰好占满配额边界：允许
func TestUploadToFolder_QuotaExactFit(t *testing.T) {
	fx := newResourceSvc()
	ctx := context.Background()
	fx.users.add(&domain.User{ID: 1, Quota: 100, Used: 90})

	content := make([]byte, 10)
	file, header := newUpload(content, "fit.bin")
	if _, err := fx.svc.UploadToFolder(ctx, 1, nil, file, header); err != nil {
		t.Fatalf("exact-fit upload should succeed: %v", err)
	}
	u, _ := fx.users.GetByID(ctx, 1)
	if u.Used != 100 {
		t.Fatalf("used = %d, want 100", u.Used)
	}
}

// 上传到别人/已删除的文件夹：拒绝
func TestUploadToFolder_BadFolder(t *testing.T) {
	fx := newResourceSvc()
	ctx := context.Background()
	fx.users.add(&domain.User{ID: 1, Quota: 1000})
	folderSvc := NewFolderService(fx.folders)

	other, _ := folderSvc.Create(ctx, 2, "other", nil)
	trashed, _ := folderSvc.Create(ctx, 1, "trash", nil)
	_ = fx.folders.SoftDelete(ctx, trashed.ID)

	for _, id := range []int64{other.ID, trashed.ID, 999} {
		file, header := newUpload([]byte("x"), "x.txt")
		if _, err := fx.svc.UploadToFolder(ctx, 1, &id, file, header); err == nil {
			t.Fatalf("expected error uploading into folder %d", id)
		}
	}
}

func TestRenameFile(t *testing.T) {
	fx := newResourceSvc()
	ctx := context.Background()
	owner := int64(1)
	res := &domain.Resource{OwnerID: &owner, OriginalName: "a.txt", Title: "a.txt", Status: domain.ResourceStatusApproved}
	_ = fx.resRepo.Create(ctx, res)

	if _, err := fx.svc.RenameFile(ctx, 1, res.ID, ""); err == nil {
		t.Fatal("expected error for empty name")
	}
	if _, err := fx.svc.RenameFile(ctx, 2, res.ID, "hack.txt"); err == nil {
		t.Fatal("expected error when renaming another user's file")
	}
	updated, err := fx.svc.RenameFile(ctx, 1, res.ID, "b.txt")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if updated.OriginalName != "b.txt" {
		t.Fatalf("name = %q, want b.txt", updated.OriginalName)
	}
}

func TestMoveFile(t *testing.T) {
	fx := newResourceSvc()
	ctx := context.Background()
	owner := int64(1)
	res := &domain.Resource{OwnerID: &owner, OriginalName: "a.txt", Status: domain.ResourceStatusApproved}
	_ = fx.resRepo.Create(ctx, res)

	folderSvc := NewFolderService(fx.folders)
	folder, _ := folderSvc.Create(ctx, 1, "docs", nil)
	other, _ := folderSvc.Create(ctx, 2, "other", nil)

	moved, err := fx.svc.MoveFile(ctx, 1, res.ID, &folder.ID)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if moved.FolderID == nil || *moved.FolderID != folder.ID {
		t.Fatal("file should be in target folder")
	}

	if _, err := fx.svc.MoveFile(ctx, 1, res.ID, &other.ID); err == nil {
		t.Fatal("expected error when target folder belongs to another user")
	}
	if _, err := fx.svc.MoveFile(ctx, 2, res.ID, nil); err == nil {
		t.Fatal("expected error when moving another user's file")
	}

	back, err := fx.svc.MoveFile(ctx, 1, res.ID, nil)
	if err != nil || back.FolderID != nil {
		t.Fatal("move back to root failed")
	}
}

func TestSoftDeleteAndDownloadPermission(t *testing.T) {
	fx := newResourceSvc()
	ctx := context.Background()
	owner := int64(1)
	res := &domain.Resource{OwnerID: &owner, OriginalName: "a.txt", Filename: "a.txt", Status: domain.ResourceStatusApproved}
	_ = fx.resRepo.Create(ctx, res)
	fx.storage.files["a.txt"] = []byte("data")

	if err := fx.svc.SoftDeleteFile(ctx, 2, res.ID); err == nil {
		t.Fatal("expected error when deleting another user's file")
	}
	if err := fx.svc.SoftDeleteFile(ctx, 1, res.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	// 回收站里的文件不能下载
	if _, _, err := fx.svc.DownloadDriveFile(ctx, 1, res.ID); err == nil {
		t.Fatal("trashed file should not be downloadable")
	}
}
