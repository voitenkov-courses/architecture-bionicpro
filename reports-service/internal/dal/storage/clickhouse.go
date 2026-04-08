package storage

import (
	"context"
	"crypto/tls"
	"fmt"
	"strconv"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/voitenkov-courses/architecture-bionicpro/reports-service/internal/config"
	"github.com/voitenkov-courses/architecture-bionicpro/reports-service/internal/services/reports"
)

type Storage struct {
	host  string
	port  string
	db    string
	table string
	conn  driver.Conn
}

func New(cfg *config.Config) *Storage {
	return &Storage{
		host:  cfg.Storage.Host,
		port:  cfg.Storage.Port,
		db:    cfg.Storage.DB,
		table: cfg.Storage.Table,
	}
}

func (s *Storage) GetReportsByUserID(ctx context.Context, userID uint32) (*reports.UserReports, error) {
	reportsSlice := make([]reports.Report, 0)

	query := `SELECT
				user_id,
				customer_name,
				customer_email,
				prosthesis,
				total_signals,
				min_signal_time,
				max_signal_time,
				report_updated
			  FROM $1
			  WHERE	user_id = $2
			  ORDER BY prosthesis_type`
	rows, err := s.conn.Query(ctx, query, s.table, strconv.Itoa(int(userID)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var (
		customerName, customerEmail string
		reportUpdatedAt             time.Time
	)

	for rows.Next() {
		var report reports.Report
		err := rows.Scan(&userID, &customerName, &customerEmail, &report.Prostesis,
			&report.TotalSignals, &report.MinSignalTime,
			&report.MaxSignalTime, &reportUpdatedAt)
		if err != nil {
			return nil, err
		}

		reportsSlice = append(reportsSlice, report)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &reports.UserReports{
		UserID:          userID,
		CustomerName:    customerName,
		CustomerEmail:   customerEmail,
		ReportUpdatedAt: reportUpdatedAt,
		Prostheses:      reportsSlice,
	}, nil
}

func (s *Storage) Connect(ctx context.Context) error {
	var err error
	s.conn, err = clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%s", s.host, s.port)},
		Auth: clickhouse.Auth{
			Database: s.db,
		},
		ClientInfo: clickhouse.ClientInfo{
			Products: []struct {
				Name    string
				Version string
			}{
				{Name: "reports-service-go-client", Version: "0.1"},
			},
		},
		Debugf: func(format string, v ...interface{}) {
			fmt.Printf(format, v)
		},
		TLS: &tls.Config{
			InsecureSkipVerify: true,
		},
	})

	if err != nil {
		return err
	}

	if err = s.conn.Ping(ctx); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			fmt.Printf("Exception [%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace)
		}
		return err
	}
	return nil
}

func (s *Storage) Close() error {
	return s.conn.Close()
}
