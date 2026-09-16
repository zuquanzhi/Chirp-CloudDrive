package service

import (
	"context"
	"testing"
	"time"

	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

// ---- 秒传 / 去重 ----

// 相同内容第二次上传：复用物理文件、秒传成功、配额按逻辑文件各记一次
func TestInstantUpload_Dedup(t *testing.T) {
	fx := newResourceSvc()
	ctx := context.Background()
	fx.users.add(&domain.User{ID: 1, Quota: 1 << 20})

	content := []byte("shared content bytes")
	f1, h1 := newUpload(content, "a.txt")
	res1, err := fx.svc.UploadToFolder(ctx, 1, nil, f1, h1)
	if err != nil {
		t.Fatalf("first upload: %v", err)
	}

	// 第二次上传同样内容（不同文件名）→ 应命中秒传
	f2, h2 := newUpload(content, "b.txt")
	res2, instant, err := fx.svc.UploadToFolderEx(ctx, 1, nil, f2, h2)
	if err != nil {
		t.Fatalf("second upload: %v", err)
	}
	if !instant {
		t.Fatal("expected instant upload (dedup hit)")
	}
	if res2.Filename != res1.Filename {
		t.Fatalf("expected shared physical file %q, got %q", res1.Filename, res2.Filename)
	}

	u, _ := fx.users.GetByID(ctx, 1)
	want := 2 * int64(len(content))
	if u.Used != want {
		t.Fatalf("quota used = %d, want %d (one logical charge per file)", u.Used, want)
	}
}

// InstantUpload 在 hash 不存在时应报错
func TestInstantUpload_UnknownHash(t *testing.T) {
	fx := newResourceSvc()
	ctx := context.Background()
	fx.users.add(&domain.User{ID: 1, Quota: 1 << 20})

	if _, err := fx.svc.InstantUpload(ctx, 1, nil, "x.txt", "deadbeef", 10); err == nil {
		t.Fatal("expected error for unknown hash")
	}
}

// 去重文件被彻底删除时，物理文件在引用计数归零前必须保留
func TestHardDelete_RefcountKeepsSharedObject(t *testing.T) {
	fx := newResourceSvc()
	ctx := context.Background()
	fx.users.add(&domain.User{ID: 1, Quota: 1 << 20})
	trash := NewTrashService(fx.folders, fx.resRepo, fx.users, fx.storage)

	content := []byte("dedup me")
	f1, h1 := newUpload(content, "one.txt")
	res1, _ := fx.svc.UploadToFolder(ctx, 1, nil, f1, h1)
	f2, h2 := newUpload(content, "two.txt")
	res2, _ := fx.svc.UploadToFolder(ctx, 1, nil, f2, h2)

	// 删除并彻底删除第一个 → 物理文件必须保留（res2 仍引用）
	if err := fx.svc.SoftDeleteFile(ctx, 1, res1.ID); err != nil {
		t.Fatal(err)
	}
	if err := trash.HardDeleteFile(ctx, 1, res1.ID); err != nil {
		t.Fatal(err)
	}
	if fx.storage.wasDeleted(res1.Filename) {
		t.Fatal("physical object deleted while still referenced")
	}

	// 彻底删除第二个 → 引用归零，物理文件删除
	if err := fx.svc.SoftDeleteFile(ctx, 1, res2.ID); err != nil {
		t.Fatal(err)
	}
	if err := trash.HardDeleteFile(ctx, 1, res2.ID); err != nil {
		t.Fatal(err)
	}
	if !fx.storage.wasDeleted(res1.Filename) {
		t.Fatal("physical object should be deleted after last reference is gone")
	}
}

// ---- 版本历史 ----

// 同名上传产生新版本，列表只保留最新版，旧版本可恢复
func TestVersionHistory(t *testing.T) {
	fx := newResourceSvc()
	ctx := context.Background()
	fx.users.add(&domain.User{ID: 1, Quota: 1 << 20})

	f1, h1 := newUpload([]byte("v1 content"), "doc.txt")
	res1, err := fx.svc.UploadToFolder(ctx, 1, nil, f1, h1)
	if err != nil {
		t.Fatal(err)
	}
	f2, h2 := newUpload([]byte("v2 content!"), "doc.txt")
	res2, err := fx.svc.UploadToFolder(ctx, 1, nil, f2, h2)
	if err != nil {
		t.Fatal(err)
	}

	if res2.VersionGroup == "" || res2.Version != 2 || !res2.IsLatest {
		t.Fatalf("expected v2 latest in a group, got %+v", res2)
	}

	// 目录列表只有最新版
	files, _ := fx.svc.ListDriveFiles(ctx, 1, nil, "")
	if len(files) != 1 || files[0].ID != res2.ID {
		t.Fatalf("expected only latest version listed, got %+v", files)
	}

	// 版本列表含两个版本
	versions, err := fx.svc.ListVersions(ctx, 1, res2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 || versions[0].Version != 2 || versions[1].Version != 1 {
		t.Fatalf("unexpected versions: %+v", versions)
	}

	// 恢复 v1 → 产生 v3，内容与 v1 相同（复用物理文件）
	res3, err := fx.svc.RestoreVersion(ctx, 1, res2.ID, res1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res3.Version != 3 || !res3.IsLatest {
		t.Fatalf("expected v3 latest, got %+v", res3)
	}
	if res3.Filename != res1.Filename {
		t.Fatal("restored version should reuse v1 physical object")
	}
}

// ---- 回收站自动清理 ----

// 超过保留期的文件/文件夹被清理且配额回收；未过期的不动
func TestCleanupExpired(t *testing.T) {
	fx := newResourceSvc()
	ctx := context.Background()
	fx.users.add(&domain.User{ID: 1, Quota: 1 << 20})
	trash := NewTrashService(fx.folders, fx.resRepo, fx.users, fx.storage)

	f1, h1 := newUpload([]byte("old file"), "old.txt")
	res1, _ := fx.svc.UploadToFolder(ctx, 1, nil, f1, h1)
	f2, h2 := newUpload([]byte("new file"), "new.txt")
	res2, _ := fx.svc.UploadToFolder(ctx, 1, nil, f2, h2)

	// 两个都进回收站，但 old.txt 的删除时间拨到 31 天前
	_ = fx.svc.SoftDeleteFile(ctx, 1, res1.ID)
	_ = fx.svc.SoftDeleteFile(ctx, 1, res2.ID)
	old := time.Now().Add(-31 * 24 * time.Hour)
	fx.resRepo.resources[res1.ID].DeletedAt = &old

	files, _, err := trash.CleanupExpired(ctx, TrashRetention)
	if err != nil {
		t.Fatal(err)
	}
	if files != 1 {
		t.Fatalf("expected 1 expired file cleaned, got %d", files)
	}
	if !fx.storage.wasDeleted(res1.Filename) {
		t.Fatal("expired physical file not removed")
	}
	// 未过期的仍在回收站
	trashed, _ := trash.ListTrashFiles(ctx, 1)
	if len(trashed) != 1 || trashed[0].ID != res2.ID {
		t.Fatalf("fresh trashed file should remain, got %+v", trashed)
	}
	// 配额只回收过期的部分
	u, _ := fx.users.GetByID(ctx, 1)
	if u.Used != int64(len("new file")) {
		t.Fatalf("quota used = %d, want %d", u.Used, len("new file"))
	}
}

// ---- 分享链接 ----

type shareFixture struct {
	svc     *ShareService
	shares  *fakeShareRepo
	fx      *resourceFixture
}

func newShareSvc() *shareFixture {
	fx := newResourceSvc()
	shares := &fakeShareRepo{byToken: map[string]*domain.Share{}}
	return &shareFixture{
		svc:    NewShareService(shares, fx.resRepo, fx.storage),
		shares: shares,
		fx:     fx,
	}
}

type fakeShareRepo struct {
	byToken map[string]*domain.Share
	nextID  int64
}

func (r *fakeShareRepo) Create(_ context.Context, s *domain.Share) error {
	r.nextID++
	s.ID = r.nextID
	cp := *s
	r.byToken[s.Token] = &cp
	return nil
}
func (r *fakeShareRepo) GetByToken(_ context.Context, token string) (*domain.Share, error) {
	if s, ok := r.byToken[token]; ok {
		cp := *s
		cp.HasPassword = cp.Password != ""
		return &cp, nil
	}
	return nil, nil
}
func (r *fakeShareRepo) ListByOwner(_ context.Context, ownerID int64) ([]domain.Share, error) {
	out := make([]domain.Share, 0)
	for _, s := range r.byToken {
		if s.OwnerID == ownerID {
			cp := *s
			cp.HasPassword = cp.Password != ""
			out = append(out, cp)
		}
	}
	return out, nil
}
func (r *fakeShareRepo) Delete(_ context.Context, id int64, ownerID int64) error {
	for token, s := range r.byToken {
		if s.ID == id && s.OwnerID == ownerID {
			delete(r.byToken, token)
		}
	}
	return nil
}
func (r *fakeShareRepo) IncrementDownloads(_ context.Context, id int64) error {
	for _, s := range r.byToken {
		if s.ID == id {
			s.Downloads++
		}
	}
	return nil
}

// 分享创建 → 信息查询 → 提取码校验 → 下载计数 → 取消后失效
func TestShareLifecycle(t *testing.T) {
	sf := newShareSvc()
	ctx := context.Background()
	sf.fx.users.add(&domain.User{ID: 1, Quota: 1 << 20})

	f, h := newUpload([]byte("share me"), "report.pdf")
	res, err := sf.fx.svc.UploadToFolder(ctx, 1, nil, f, h)
	if err != nil {
		t.Fatal(err)
	}

	share, err := sf.svc.Create(ctx, 1, res.ID, "1234", 7)
	if err != nil {
		t.Fatal(err)
	}
	if share.Token == "" || share.ExpiresAt == nil {
		t.Fatalf("bad share: %+v", share)
	}

	// 信息查询
	info, file, err := sf.svc.GetInfo(ctx, share.Token)
	if err != nil {
		t.Fatal(err)
	}
	if !info.HasPassword || file.OriginalName != "report.pdf" {
		t.Fatalf("bad share info: %+v %+v", info, file)
	}

	// 错误提取码
	if _, _, err := sf.svc.Download(ctx, share.Token, "0000"); err == nil {
		t.Fatal("expected wrong extraction code error")
	}
	// 正确提取码 → 下载成功且计数 +1
	_, reader, err := sf.svc.Download(ctx, share.Token, "1234")
	if err != nil {
		t.Fatal(err)
	}
	reader.Close()
	if got := sf.shares.byToken[share.Token].Downloads; got != 1 {
		t.Fatalf("downloads = %d, want 1", got)
	}

	// 取消分享
	if err := sf.svc.Cancel(ctx, 1, share.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := sf.svc.GetInfo(ctx, share.Token); err == nil {
		t.Fatal("share should be gone after cancel")
	}
}

// 过期分享不可访问
func TestShareExpired(t *testing.T) {
	sf := newShareSvc()
	ctx := context.Background()
	sf.fx.users.add(&domain.User{ID: 1, Quota: 1 << 20})

	f, h := newUpload([]byte("x"), "x.txt")
	res, _ := sf.fx.svc.UploadToFolder(ctx, 1, nil, f, h)
	share, err := sf.svc.Create(ctx, 1, res.ID, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	sf.shares.byToken[share.Token].ExpiresAt = &past

	if _, _, err := sf.svc.GetInfo(ctx, share.Token); err == nil || err.Error() != "share expired" {
		t.Fatalf("expected share expired, got %v", err)
	}
}
