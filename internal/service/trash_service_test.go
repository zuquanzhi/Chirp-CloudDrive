package service

import (
	"context"
	"testing"

	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

type trashFixture struct {
	svc     *TrashService
	folders *fakeFolderRepo
	resRepo *fakeResourceRepo
	users   *fakeUserRepo
	storage *fakeStorage
}

func newTrashSvc() *trashFixture {
	folders := newFakeFolderRepo()
	resRepo := newFakeResourceRepo()
	users := newFakeUserRepo()
	storage := newFakeStorage()
	return &trashFixture{
		svc:     NewTrashService(folders, resRepo, users, storage),
		folders: folders,
		resRepo: resRepo,
		users:   users,
		storage: storage,
	}
}

func (fx *trashFixture) addFolder(t *testing.T, owner int64, name string, parent *int64) int64 {
	t.Helper()
	ctx := context.Background()
	f := &domain.Folder{OwnerID: owner, Name: name, ParentID: parent}
	if err := fx.folders.Create(ctx, f); err != nil {
		t.Fatal(err)
	}
	return f.ID
}

func (fx *trashFixture) addFile(t *testing.T, owner int64, name string, folder *int64, size int64) int64 {
	t.Helper()
	ctx := context.Background()
	res := &domain.Resource{
		OwnerID:      &owner,
		FolderID:     folder,
		OriginalName: name,
		Filename:     name,
		Size:         size,
		Status:       domain.ResourceStatusApproved,
	}
	if err := fx.resRepo.Create(ctx, res); err != nil {
		t.Fatal(err)
	}
	fx.storage.files[name] = make([]byte, size)
	return res.ID
}

// 级联软删：A(B(file1), file2) 删 A → 子文件夹和子树里所有文件都进回收站
func TestDeleteFolder_Cascade(t *testing.T) {
	fx := newTrashSvc()
	ctx := context.Background()
	a := fx.addFolder(t, 1, "A", nil)
	b := fx.addFolder(t, 1, "B", &a)
	fileInB := fx.addFile(t, 1, "f1.txt", &b, 10)
	fileInA := fx.addFile(t, 1, "f2.txt", &a, 10)
	outside := fx.addFile(t, 1, "keep.txt", nil, 10)

	if err := fx.svc.DeleteFolder(ctx, 1, a); err != nil {
		t.Fatalf("delete folder: %v", err)
	}

	for _, id := range []int64{a, b} {
		f, _ := fx.folders.GetByID(ctx, id)
		if f.DeletedAt == nil {
			t.Fatalf("folder %d should be trashed", id)
		}
	}
	for _, id := range []int64{fileInB, fileInA} {
		res, _ := fx.resRepo.GetByID(ctx, id)
		if res.DeletedAt == nil {
			t.Fatalf("file %d should be trashed", id)
		}
	}
	keep, _ := fx.resRepo.GetByID(ctx, outside)
	if keep.DeletedAt != nil {
		t.Fatal("file outside the subtree must survive")
	}

	// 不能删别人的文件夹
	if err := fx.svc.DeleteFolder(ctx, 2, a); err == nil {
		t.Fatal("expected error deleting another user's folder")
	}
}

// 回收站列表只显示顶层项：父已删的子文件夹、在已删文件夹里的文件都不单独出现
func TestListTrash_TopLevelOnly(t *testing.T) {
	fx := newTrashSvc()
	ctx := context.Background()
	a := fx.addFolder(t, 1, "A", nil)
	b := fx.addFolder(t, 1, "B", &a)
	fx.addFile(t, 1, "inside.txt", &a, 5)
	loose := fx.addFile(t, 1, "loose.txt", nil, 5)

	_ = fx.svc.DeleteFolder(ctx, 1, a)
	_ = fx.resRepo.SoftDelete(ctx, loose)

	folders, err := fx.svc.ListTrashFolders(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 1 || folders[0].ID != a {
		t.Fatalf("expected only top folder A in trash, got %+v", folders)
	}
	_ = b

	files, err := fx.svc.ListTrashFiles(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].ID != loose {
		t.Fatalf("expected only loose file in trash, got %+v", files)
	}
	if files[0].URL == "" {
		t.Fatal("trash file should carry a preview URL")
	}
}

// 还原整个子树
func TestRestoreFolder_Cascade(t *testing.T) {
	fx := newTrashSvc()
	ctx := context.Background()
	a := fx.addFolder(t, 1, "A", nil)
	b := fx.addFolder(t, 1, "B", &a)
	fileInB := fx.addFile(t, 1, "f1.txt", &b, 10)

	_ = fx.svc.DeleteFolder(ctx, 1, a)
	if err := fx.svc.RestoreFolder(ctx, 1, a); err != nil {
		t.Fatalf("restore: %v", err)
	}

	for _, id := range []int64{a, b} {
		f, _ := fx.folders.GetByID(ctx, id)
		if f.DeletedAt != nil {
			t.Fatalf("folder %d should be restored", id)
		}
	}
	res, _ := fx.resRepo.GetByID(ctx, fileInB)
	if res.DeletedAt != nil {
		t.Fatal("file in subtree should be restored")
	}

	// 不在回收站里的文件夹不能还原
	if err := fx.svc.RestoreFolder(ctx, 1, a); err == nil {
		t.Fatal("expected error restoring a non-trashed folder")
	}
}

// 父文件夹也被删时，还原的子项重新挂到根目录
func TestRestore_ReattachToRoot(t *testing.T) {
	fx := newTrashSvc()
	ctx := context.Background()
	p := fx.addFolder(t, 1, "P", nil)
	c := fx.addFolder(t, 1, "C", &p)
	fileInC := fx.addFile(t, 1, "f.txt", &c, 5)

	// 父和子分别进回收站（模拟先删子、再删父的场景）
	_ = fx.folders.SoftDelete(ctx, c)
	_ = fx.svc.DeleteFolder(ctx, 1, p) // 会级联再删 c（幂等）

	// 还原父：子随子树一起还原
	if err := fx.svc.RestoreFolder(ctx, 1, p); err != nil {
		t.Fatal(err)
	}
	cf, _ := fx.folders.GetByID(ctx, c)
	if cf.DeletedAt != nil || cf.ParentID == nil || *cf.ParentID != p {
		t.Fatal("child should be restored under its parent")
	}

	// 反向场景：父仍在回收站时单独还原子 → 子挂到根
	_ = fx.svc.DeleteFolder(ctx, 1, p)
	if err := fx.svc.RestoreFolder(ctx, 1, c); err != nil {
		t.Fatal(err)
	}
	cf, _ = fx.folders.GetByID(ctx, c)
	if cf.DeletedAt != nil || cf.ParentID != nil {
		t.Fatal("orphaned child should be re-attached to root")
	}

	// 文件所在文件夹已删 → 文件还原到根
	_ = fx.folders.SoftDelete(ctx, c) // 此时 c 已还原到根，单独把它再删掉
	_ = fx.resRepo.SoftDelete(ctx, fileInC)
	if err := fx.svc.RestoreFile(ctx, 1, fileInC); err != nil {
		t.Fatal(err)
	}
	res, _ := fx.resRepo.GetByID(ctx, fileInC)
	if res.DeletedAt != nil || res.FolderID != nil {
		t.Fatal("file should be restored to root")
	}
}

// 彻底删除文件：物理文件删除 + 配额回收
func TestHardDeleteFile(t *testing.T) {
	fx := newTrashSvc()
	ctx := context.Background()
	fx.users.add(&domain.User{ID: 1, Quota: 1000, Used: 500})
	id := fx.addFile(t, 1, "gone.txt", nil, 200)
	_ = fx.resRepo.SoftDelete(ctx, id)

	if err := fx.svc.HardDeleteFile(ctx, 1, id); err != nil {
		t.Fatalf("hard delete: %v", err)
	}

	if res, _ := fx.resRepo.GetByID(ctx, id); res != nil {
		t.Fatal("row should be gone")
	}
	if !fx.storage.wasDeleted("gone.txt") {
		t.Fatal("physical file should be deleted")
	}
	u, _ := fx.users.GetByID(ctx, 1)
	if u.Used != 300 {
		t.Fatalf("used = %d, want 300", u.Used)
	}

	// 未进回收站的文件不能彻底删
	alive := fx.addFile(t, 1, "alive.txt", nil, 1)
	if err := fx.svc.HardDeleteFile(ctx, 1, alive); err == nil {
		t.Fatal("expected error hard-deleting a non-trashed file")
	}
}

// 彻底删除文件夹：整个子树的物理文件删除 + 配额一次性回收
func TestHardDeleteFolder(t *testing.T) {
	fx := newTrashSvc()
	ctx := context.Background()
	fx.users.add(&domain.User{ID: 1, Quota: 1000, Used: 600})
	a := fx.addFolder(t, 1, "A", nil)
	b := fx.addFolder(t, 1, "B", &a)
	fx.addFile(t, 1, "f1.txt", &a, 100)
	fx.addFile(t, 1, "f2.txt", &b, 200)

	// 没进回收站不能彻底删
	if err := fx.svc.HardDeleteFolder(ctx, 1, a); err == nil {
		t.Fatal("expected error hard-deleting a non-trashed folder")
	}

	_ = fx.svc.DeleteFolder(ctx, 1, a)
	if err := fx.svc.HardDeleteFolder(ctx, 1, a); err != nil {
		t.Fatalf("hard delete folder: %v", err)
	}

	for _, id := range []int64{a, b} {
		if f, _ := fx.folders.GetByID(ctx, id); f != nil {
			t.Fatalf("folder %d should be gone", id)
		}
	}
	if len(fx.resRepo.resources) != 0 {
		t.Fatal("all file rows in subtree should be gone")
	}
	for _, name := range []string{"f1.txt", "f2.txt"} {
		if !fx.storage.wasDeleted(name) {
			t.Fatalf("physical file %s should be deleted", name)
		}
	}
	u, _ := fx.users.GetByID(ctx, 1)
	if u.Used != 300 {
		t.Fatalf("used = %d, want 300 (600-100-200)", u.Used)
	}
}
