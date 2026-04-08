package reports

import (
	"context"
	"encoding/json"
	"time"
)

type ReportsService struct {
	storage Storage
	cdn     Cdn
}

type UserReports struct {
	UserID          uint32    `json:"userID"`
	CustomerName    string    `json:"customerName"`
	CustomerEmail   string    `json:"customerEmail"`
	Prostheses      []Report  `json:"prosteses"`
	ReportUpdatedAt time.Time `json:"reportUpdatedAt"`
}

type Report struct {
	Prostesis     string    `json:"prostesis"`
	TotalSignals  uint64    `json:"totalSignals"`
	MinSignalTime time.Time `json:"minSignalTime"`
	MaxSignalTime time.Time `json:"maxSignalTime"`
}

type Storage interface {
	GetReportsByUserID(ctx context.Context, userID uint32) (*UserReports, error)
	Connect(ctx context.Context) error
	Close() error
}

type Cdn interface {
	InitClientAndBucket(ctx context.Context) error
	EnsureBucket(ctx context.Context) error
	BucketExists(ctx context.Context) (bool, error)
	ReportsExists(ctx context.Context, userID uint32) (bool, error)
	GetReportsByUserID(ctx context.Context, userID uint32) (*[]byte, error)
	PutReport(ctx context.Context, userID uint32, data []byte) error
	GetCdnURL(userID uint32) string
}

func New(storage Storage, cdn Cdn) *ReportsService {
	return &ReportsService{
		storage: storage,
		cdn:     cdn,
	}
}

func (rs *ReportsService) GetReportByUserID(ctx context.Context, userID uint32) (*UserReports, error) {
	userReports := &UserReports{}

	// Проверяем есть ли отчет в CDN
	reportExists, err := rs.cdn.ReportsExists(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Читаем отчет из бакета S3
	if reportExists {
		cachedReports, err := rs.cdn.GetReportsByUserID(ctx, userID)
		if err != nil {
			return nil, err
		}

		if err = json.Unmarshal(*cachedReports, userReports); err != nil {
			return nil, err
		}

		return userReports, nil
	}

	// Если отчета нет в CDN, достаем из базы
	userReports, err = rs.storage.GetReportsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Кладем отчет в S3
	json, err := json.Marshal(*userReports)
	if err == nil {
		rs.cdn.PutReport(ctx, userID, json)
	}

	return userReports, nil
}
