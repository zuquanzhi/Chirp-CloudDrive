package service

import (
	"bytes"
	"context"
	"io"
	"strings"
	"time"

	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

// ---- helpers ----

func ptrInt64(v int64) *int64 { return &v }

func ptrEqual(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// ---- fake FolderRepository ----

type fakeFolderRepo struct {
	folders map[int64]*domain.Folder
	nextID  int64
}

func newFakeFolderRepo() *fakeFolderRepo {
	return &fakeFolderRepo{folders: make(map[int64]*domain.Folder), nextID: 1}
}

func (r *fakeFolderRepo) Create(_ context.Context, f *domain.Folder) error {
	f.ID = r.nextID
	r.nextID++
	cp := *f
	r.folders[f.ID] = &cp
	return nil
}

func (r *fakeFolderRepo) GetByID(_ context.Context, id int64) (*domain.Folder, error) {
	if f, ok := r.folders[id]; ok {
		cp := *f
		return &cp, nil
	}
	return nil, nil
}

func (r *fakeFolderRepo) ListByParent(_ context.Context, ownerID int64, parentID *int64) ([]domain.Folder, error) {
	out := make([]domain.Folder, 0)
	for _, f := range r.folders {
		if f.OwnerID == ownerID && f.DeletedAt == nil && ptrEqual(f.ParentID, parentID) {
			out = append(out, *f)
		}
	}
	return out, nil
}

func (r *fakeFolderRepo) Update(_ context.Context, f *domain.Folder) error {
	if _, ok := r.folders[f.ID]; !ok {
		return nil
	}
	cp := *f
	r.folders[f.ID] = &cp
	return nil
}

func (r *fakeFolderRepo) SoftDelete(_ context.Context, id int64) error {
	if f, ok := r.folders[id]; ok {
		now := time.Now()
		f.DeletedAt = &now
	}
	return nil
}

// ListDescendantIDs returns all descendant ids regardless of soft-delete state,
// mirroring the sqlite implementation used by trash cascade and cycle checks.
func (r *fakeFolderRepo) ListDescendantIDs(_ context.Context, id int64) ([]int64, error) {
	var out []int64
	queue := []int64{id}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, f := range r.folders {
			if f.ParentID != nil && *f.ParentID == cur {
				out = append(out, f.ID)
				queue = append(queue, f.ID)
			}
		}
	}
	return out, nil
}

func (r *fakeFolderRepo) ListDeleted(_ context.Context, ownerID int64) ([]domain.Folder, error) {
	out := make([]domain.Folder, 0)
	for _, f := range r.folders {
		if f.OwnerID == ownerID && f.DeletedAt != nil {
			out = append(out, *f)
		}
	}
	return out, nil
}

func (r *fakeFolderRepo) Restore(_ context.Context, id int64) error {
	if f, ok := r.folders[id]; ok {
		f.DeletedAt = nil
	}
	return nil
}

func (r *fakeFolderRepo) HardDelete(_ context.Context, id int64) error {
	delete(r.folders, id)
	return nil
}

// ---- fake ResourceRepository ----

type fakeResourceRepo struct {
	resources map[int64]*domain.Resource
	nextID    int64
}

func newFakeResourceRepo() *fakeResourceRepo {
	return &fakeResourceRepo{resources: make(map[int64]*domain.Resource), nextID: 1}
}

func (r *fakeResourceRepo) Create(_ context.Context, res *domain.Resource) error {
	res.ID = r.nextID
	r.nextID++
	cp := *res
	r.resources[res.ID] = &cp
	return nil
}

func (r *fakeResourceRepo) List(_ context.Context, status domain.ResourceStatus, search string) ([]domain.Resource, error) {
	out := make([]domain.Resource, 0)
	for _, res := range r.resources {
		if res.Status == status && (search == "" || strings.Contains(res.Title, search)) {
			out = append(out, *res)
		}
	}
	return out, nil
}

func (r *fakeResourceRepo) GetByID(_ context.Context, id int64) (*domain.Resource, error) {
	if res, ok := r.resources[id]; ok {
		cp := *res
		return &cp, nil
	}
	return nil, nil
}

func (r *fakeResourceRepo) UpdateStatus(_ context.Context, id int64, status domain.ResourceStatus) error {
	if res, ok := r.resources[id]; ok {
		res.Status = status
	}
	return nil
}

func (r *fakeResourceRepo) GetByHash(_ context.Context, hash string) ([]domain.Resource, error) {
	out := make([]domain.Resource, 0)
	for _, res := range r.resources {
		if res.FileHash == hash {
			out = append(out, *res)
		}
	}
	return out, nil
}

func (r *fakeResourceRepo) ListByFolder(_ context.Context, ownerID int64, folderID *int64, search string) ([]domain.Resource, error) {
	out := make([]domain.Resource, 0)
	for _, res := range r.resources {
		if res.OwnerID == nil || *res.OwnerID != ownerID || res.DeletedAt != nil {
			continue
		}
		if !ptrEqual(res.FolderID, folderID) {
			continue
		}
		if search != "" && !strings.Contains(res.OriginalName, search) {
			continue
		}
		out = append(out, *res)
	}
	return out, nil
}

func (r *fakeResourceRepo) Update(_ context.Context, res *domain.Resource) error {
	if cur, ok := r.resources[res.ID]; ok {
		cur.OriginalName = res.OriginalName
		cur.Title = res.Title
		cur.FolderID = res.FolderID
	}
	return nil
}

func (r *fakeResourceRepo) SoftDelete(_ context.Context, id int64) error {
	if res, ok := r.resources[id]; ok {
		now := time.Now()
		res.DeletedAt = &now
	}
	return nil
}

func (r *fakeResourceRepo) SoftDeleteByFolder(_ context.Context, folderID int64) error {
	now := time.Now()
	for _, res := range r.resources {
		if res.FolderID != nil && *res.FolderID == folderID && res.DeletedAt == nil {
			res.DeletedAt = &now
		}
	}
	return nil
}

func (r *fakeResourceRepo) ListDeleted(_ context.Context, ownerID int64) ([]domain.Resource, error) {
	out := make([]domain.Resource, 0)
	for _, res := range r.resources {
		if res.OwnerID != nil && *res.OwnerID == ownerID && res.DeletedAt != nil {
			out = append(out, *res)
		}
	}
	return out, nil
}

func (r *fakeResourceRepo) ListByFolderIncludingDeleted(_ context.Context, folderID int64) ([]domain.Resource, error) {
	out := make([]domain.Resource, 0)
	for _, res := range r.resources {
		if res.FolderID != nil && *res.FolderID == folderID {
			out = append(out, *res)
		}
	}
	return out, nil
}

func (r *fakeResourceRepo) Restore(_ context.Context, id int64, folderID *int64) error {
	if res, ok := r.resources[id]; ok {
		res.DeletedAt = nil
		res.FolderID = folderID
	}
	return nil
}

func (r *fakeResourceRepo) RestoreByFolder(_ context.Context, folderID int64) error {
	for _, res := range r.resources {
		if res.FolderID != nil && *res.FolderID == folderID {
			res.DeletedAt = nil
		}
	}
	return nil
}

func (r *fakeResourceRepo) HardDelete(_ context.Context, id int64) error {
	delete(r.resources, id)
	return nil
}

func (r *fakeResourceRepo) HardDeleteByFolder(_ context.Context, folderID int64) error {
	for id, res := range r.resources {
		if res.FolderID != nil && *res.FolderID == folderID {
			delete(r.resources, id)
		}
	}
	return nil
}

// ---- fake UserRepository ----

type fakeUserRepo struct {
	users map[int64]*domain.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: make(map[int64]*domain.User)}
}

func (r *fakeUserRepo) add(u *domain.User) {
	cp := *u
	r.users[u.ID] = &cp
}

func (r *fakeUserRepo) Create(_ context.Context, u *domain.User) error {
	r.users[u.ID] = u
	return nil
}

func (r *fakeUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	for _, u := range r.users {
		if u.Email == email {
			cp := *u
			return &cp, nil
		}
	}
	return nil, nil
}

func (r *fakeUserRepo) GetByID(_ context.Context, id int64) (*domain.User, error) {
	if u, ok := r.users[id]; ok {
		cp := *u
		return &cp, nil
	}
	return nil, nil
}

func (r *fakeUserRepo) UpdateProfile(_ context.Context, u *domain.User) error {
	if cur, ok := r.users[u.ID]; ok {
		*cur = *u
	}
	return nil
}

func (r *fakeUserRepo) AddUsed(_ context.Context, userID int64, delta int64) error {
	if u, ok := r.users[userID]; ok {
		u.Used += delta
	}
	return nil
}

// ---- fake FileStorage ----

type fakeStorage struct {
	files   map[string][]byte
	deleted []string
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{files: make(map[string][]byte)}
}

func (s *fakeStorage) Save(_ context.Context, file io.Reader, filename string) (string, int64, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return "", 0, err
	}
	s.files[filename] = data
	return filename, int64(len(data)), nil
}

func (s *fakeStorage) Get(_ context.Context, path string) (io.ReadCloser, error) {
	data, ok := s.files[path]
	if !ok {
		return nil, io.ErrClosedPipe
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *fakeStorage) Delete(_ context.Context, path string) error {
	delete(s.files, path)
	s.deleted = append(s.deleted, path)
	return nil
}

func (s *fakeStorage) GetPublicURL(path string) string {
	return "/uploads/" + path
}

func (s *fakeStorage) wasDeleted(path string) bool {
	for _, d := range s.deleted {
		if d == path {
			return true
		}
	}
	return false
}
