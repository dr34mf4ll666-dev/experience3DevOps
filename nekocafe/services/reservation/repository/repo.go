package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Reservation struct {
	ID        string    `json:"id"`
	MemberID  string    `json:"member_id"`
	StoreID   string    `json:"store_id"`
	Slot      time.Time `json:"slot"`
	Seats     int       `json:"seats"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type Repo struct {
	pg    *pgxpool.Pool
	cache *redis.Client
}

func New(dsn, redisAddr string) (*Repo, error) {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("pgx 连接失败: %w", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})

	_, err = pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS reservations (
			id TEXT PRIMARY KEY,
			member_id TEXT NOT NULL,
			store_id TEXT NOT NULL,
			slot TIMESTAMPTZ NOT NULL,
			seats INT NOT NULL CHECK (seats > 0 AND seats <= 10),
			status TEXT NOT NULL DEFAULT 'pending',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_res_member ON reservations(member_id);
		CREATE INDEX IF NOT EXISTS idx_res_store_slot ON reservations(store_id, slot);
	`)
	if err != nil {
		return nil, fmt.Errorf("初始化表失败: %w", err)
	}
	return &Repo{pg: pool, cache: rdb}, nil
}

func (r *Repo) Close() {
	r.pg.Close()
	_ = r.cache.Close()
}

func (r *Repo) Create(ctx context.Context, in *Reservation) error {
	in.ID = uuid.NewString()
	in.Status = "confirmed"
	in.CreatedAt = time.Now()

	_, err := r.pg.Exec(ctx, `
		INSERT INTO reservations(id, member_id, store_id, slot, seats, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, in.ID, in.MemberID, in.StoreID, in.Slot, in.Seats, in.Status, in.CreatedAt)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("nekocafe:store:%s:slot:%s", in.StoreID, in.Slot.Format(time.RFC3339))
	r.cache.IncrBy(ctx, key, int64(in.Seats))
	r.cache.Expire(ctx, key, 24*time.Hour)
	return nil
}

func (r *Repo) Get(ctx context.Context, id string) (*Reservation, error) {
	row := r.pg.QueryRow(ctx, `
		SELECT id, member_id, store_id, slot, seats, status, created_at
		FROM reservations WHERE id=$1
	`, id)
	var res Reservation
	if err := row.Scan(&res.ID, &res.MemberID, &res.StoreID, &res.Slot,
		&res.Seats, &res.Status, &res.CreatedAt); err != nil {
		return nil, err
	}
	return &res, nil
}

func (r *Repo) Ping(ctx context.Context) error {
	if err := r.pg.Ping(ctx); err != nil {
		return err
	}
	return r.cache.Ping(ctx).Err()
}
