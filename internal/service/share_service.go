package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"time"

	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

// ShareService manages public share links for drive files.
type ShareService struct {
	shareRepo    domain.ShareRepository
	resourceRepo domain.ResourceRepository
	storage      FileStorage
	activity     *ActivityRecorder
}

func NewShareService(shareRepo domain.ShareRepository, resourceRepo domain.ResourceRepository, storage FileStorage) *ShareService {
	return &ShareService{shareRepo: shareRepo, resourceRepo: resourceRepo, storage: storage}
}

func (s *ShareService) SetActivityRecorder(rec *ActivityRecorder) {
	s.activity = rec
}

// Create makes a share link for a file the user owns. expireDays <= 0 means never expires.
func (s *ShareService) Create(ctx context.Context, ownerID, resourceID int64, password string, expireDays int) (*domain.Share, error) {
	res, err := s.resourceRepo.GetByID(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	if res == nil || res.OwnerID == nil || *res.OwnerID != ownerID || res.DeletedAt != nil {
		return nil, errors.New("file not found")
	}

	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	share := &domain.Share{
		Token:      hex.EncodeToString(buf),
		ResourceID: resourceID,
		OwnerID:    ownerID,
		Password:   password,
	}
	if expireDays > 0 {
		t := time.Now().Add(time.Duration(expireDays) * 24 * time.Hour)
		share.ExpiresAt = &t
	}
	if err := s.shareRepo.Create(ctx, share); err != nil {
		return nil, err
	}
	share.HasPassword = password != ""
	s.activity.Log(ctx, ownerID, domain.ActivityShareCreate, "share", share.ID, res.OriginalName, "创建分享链接")
	return share, nil
}

// List returns all shares of a user, newest first.
func (s *ShareService) List(ctx context.Context, ownerID int64) ([]domain.Share, error) {
	return s.shareRepo.ListByOwner(ctx, ownerID)
}

// Cancel removes a share link owned by the user.
func (s *ShareService) Cancel(ctx context.Context, ownerID, shareID int64) error {
	if err := s.shareRepo.Delete(ctx, shareID, ownerID); err != nil {
		return err
	}
	s.activity.Log(ctx, ownerID, domain.ActivityShareCancel, "share", shareID, "", "取消分享")
	return nil
}

// GetInfo returns public info about a share (validates expiry).
func (s *ShareService) GetInfo(ctx context.Context, token string) (*domain.Share, *domain.Resource, error) {
	share, err := s.validShare(ctx, token)
	if err != nil {
		return nil, nil, err
	}
	res, err := s.resourceRepo.GetByID(ctx, share.ResourceID)
	if err != nil {
		return nil, nil, err
	}
	if res == nil || res.DeletedAt != nil {
		return nil, nil, errors.New("shared file no longer exists")
	}
	share.FileName = res.OriginalName
	share.FileSize = res.Size
	return share, res, nil
}

// Download validates password/expiry, counts the download and returns file content.
func (s *ShareService) Download(ctx context.Context, token, password string) (*domain.Resource, io.ReadCloser, error) {
	share, err := s.validShare(ctx, token)
	if err != nil {
		return nil, nil, err
	}
	if share.Password != "" && share.Password != password {
		return nil, nil, errors.New("wrong extraction code")
	}
	res, err := s.resourceRepo.GetByID(ctx, share.ResourceID)
	if err != nil {
		return nil, nil, err
	}
	if res == nil || res.DeletedAt != nil {
		return nil, nil, errors.New("shared file no longer exists")
	}
	reader, err := s.storage.Get(ctx, res.Filename)
	if err != nil {
		return nil, nil, err
	}
	if err := s.shareRepo.IncrementDownloads(ctx, share.ID); err != nil {
		reader.Close()
		return nil, nil, err
	}
	return res, reader, nil
}

func (s *ShareService) validShare(ctx context.Context, token string) (*domain.Share, error) {
	share, err := s.shareRepo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if share == nil {
		return nil, errors.New("share not found")
	}
	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		return nil, errors.New("share expired")
	}
	return share, nil
}
