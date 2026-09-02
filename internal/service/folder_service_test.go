package service

import (
	"context"
	"testing"
)

func newFolderSvcWithTree(t *testing.T) (*FolderService, *fakeFolderRepo) {
	t.Helper()
	repo := newFakeFolderRepo()
	return NewFolderService(repo), repo
}

func mustCreateFolder(t *testing.T, svc *FolderService, ownerID int64, name string, parentID *int64) int64 {
	t.Helper()
	f, err := svc.Create(context.Background(), ownerID, name, parentID)
	if err != nil {
		t.Fatalf("create folder %q: %v", name, err)
	}
	return f.ID
}

func TestFolderCreate(t *testing.T) {
	svc, _ := newFolderSvcWithTree(t)
	ctx := context.Background()

	if _, err := svc.Create(ctx, 1, "", nil); err == nil {
		t.Fatal("expected error for empty folder name")
	}

	if _, err := svc.Create(ctx, 1, "x", ptrInt64(999)); err == nil {
		t.Fatal("expected error for nonexistent parent")
	}

	f, err := svc.Create(ctx, 1, "docs", nil)
	if err != nil {
		t.Fatalf("create root folder: %v", err)
	}
	if f.ParentID != nil {
		t.Fatal("root folder should have nil parent")
	}

	// 别人的文件夹不能当父目录
	if _, err := svc.Create(ctx, 2, "evil", ptrInt64(f.ID)); err == nil {
		t.Fatal("expected error when parent belongs to another user")
	}
}

func TestFolderRename(t *testing.T) {
	svc, _ := newFolderSvcWithTree(t)
	ctx := context.Background()
	id := mustCreateFolder(t, svc, 1, "old", nil)

	f, err := svc.Rename(ctx, 1, id, "new")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if f.Name != "new" {
		t.Fatalf("expected name new, got %q", f.Name)
	}

	if _, err := svc.Rename(ctx, 1, id, ""); err == nil {
		t.Fatal("expected error for empty name")
	}
	if _, err := svc.Rename(ctx, 2, id, "hack"); err == nil {
		t.Fatal("expected error when renaming another user's folder")
	}
}

// 防环：移动到自身
func TestFolderMove_IntoItself(t *testing.T) {
	svc, _ := newFolderSvcWithTree(t)
	ctx := context.Background()
	a := mustCreateFolder(t, svc, 1, "A", nil)

	if _, err := svc.Move(ctx, 1, a, ptrInt64(a)); err == nil {
		t.Fatal("expected error when moving folder into itself")
	}
}

// 防环：移动到后代（A → B → C，把 A 移进 C）
func TestFolderMove_IntoDescendant(t *testing.T) {
	svc, _ := newFolderSvcWithTree(t)
	ctx := context.Background()
	a := mustCreateFolder(t, svc, 1, "A", nil)
	b := mustCreateFolder(t, svc, 1, "B", ptrInt64(a))
	c := mustCreateFolder(t, svc, 1, "C", ptrInt64(b))

	if _, err := svc.Move(ctx, 1, a, ptrInt64(c)); err == nil {
		t.Fatal("expected error when moving folder into its own descendant")
	}
	if _, err := svc.Move(ctx, 1, a, ptrInt64(b)); err == nil {
		t.Fatal("expected error when moving folder into direct child")
	}
}

// 合法移动：B 从 A 下移到根目录
func TestFolderMove_Valid(t *testing.T) {
	svc, repo := newFolderSvcWithTree(t)
	ctx := context.Background()
	a := mustCreateFolder(t, svc, 1, "A", nil)
	b := mustCreateFolder(t, svc, 1, "B", ptrInt64(a))

	f, err := svc.Move(ctx, 1, b, nil)
	if err != nil {
		t.Fatalf("move to root: %v", err)
	}
	if f.ParentID != nil {
		t.Fatal("expected folder re-attached to root")
	}
	stored, _ := repo.GetByID(ctx, b)
	if stored.ParentID != nil {
		t.Fatal("repo should persist the move")
	}
}

func TestFolderMove_Permission(t *testing.T) {
	svc, repo := newFakeFolderRepoTreeForPerm(t)
	ctx := context.Background()

	// 移动别人的文件夹（user 1 操作 user 2 的文件夹）
	if _, err := svc.Move(ctx, 1, repo.a, nil); err == nil {
		t.Fatal("expected error when moving another user's folder")
	}
	// 移进别人的文件夹
	if _, err := svc.Move(ctx, 1, repo.own, ptrInt64(repo.a)); err == nil {
		t.Fatal("expected error when target belongs to another user")
	}
	// 移进回收站里的文件夹
	if _, err := svc.Move(ctx, 1, repo.own, ptrInt64(repo.deleted)); err == nil {
		t.Fatal("expected error when target is in trash")
	}
}

type permTree struct {
	a       int64 // user 2 的文件夹
	own     int64 // user 1 的文件夹
	deleted int64 // user 1 已删除的文件夹
}

func newFakeFolderRepoTreeForPerm(t *testing.T) (*FolderService, *permTree) {
	t.Helper()
	repo := newFakeFolderRepo()
	svc := NewFolderService(repo)
	ctx := context.Background()

	a := mustCreateFolder(t, svc, 2, "other", nil)
	own := mustCreateFolder(t, svc, 1, "mine", nil)
	deleted := mustCreateFolder(t, svc, 1, "trashed", nil)
	if err := repo.SoftDelete(ctx, deleted); err != nil {
		t.Fatal(err)
	}
	return svc, &permTree{a: a, own: own, deleted: deleted}
}

func TestFolderList(t *testing.T) {
	svc, repo := newFolderSvcWithTree(t)
	ctx := context.Background()
	a := mustCreateFolder(t, svc, 1, "A", nil)
	mustCreateFolder(t, svc, 1, "B", ptrInt64(a))
	mustCreateFolder(t, svc, 2, "not-mine", nil)

	roots, err := svc.List(ctx, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || roots[0].Name != "A" {
		t.Fatalf("expected only folder A at root, got %+v", roots)
	}

	children, err := svc.List(ctx, 1, ptrInt64(a))
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 1 || children[0].Name != "B" {
		t.Fatalf("expected only folder B under A, got %+v", children)
	}

	// 已删除的文件夹不出现在列表里
	if err := repo.SoftDelete(ctx, a); err != nil {
		t.Fatal(err)
	}
	roots, _ = svc.List(ctx, 1, nil)
	if len(roots) != 0 {
		t.Fatal("deleted folder should not be listed")
	}
}
