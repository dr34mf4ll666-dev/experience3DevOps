// Package repository 封装预约数据访问：Postgres 持久 + Redis 缓存可用名额。
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Reservation 预约领域对象。
type Reservation struct {
	ID        string    `json:"id"`
	MemberID  string    `json:"member_id"`
	StoreID   string    `json:"store_id"`
	Slot      time.Time `json:"slot"`
	Seats     int       `json:"seats"`
	Status    string    `json:"status"` // pending / confirmed / cancelled
	CreatedAt time.Time `json:"created_at"`
}

// Repo 数据访问门面。
type Repo struct {
	pg    *pgxpool.Pool
	cache *redis.Client
}

// New 建立连接池。
func New(dsn, redisAddr string) (*Repo, error) {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("pgx 连接失败: %w", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})

	// 启动时建表（生产中应使用 migrate 工具，演示用就地建）
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

// Close 释放资源。
func (r *Repo) Close() {
	r.pg.Close()
	_ = r.cache.Close()
}

// Create 落库一条预约。
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
	// 缓存当日某门店剩余席位（演示，简化为 TTL）
	key := fmt.Sprintf("nekocafe:store:%s:slot:%s", in.StoreID, in.Slot.Format(time.RFC3339))
	r.cache.IncrBy(ctx, key, int64(in.Seats))
	r.cache.Expire(ctx, key, 24*time.Hour)
	return nil
}

// Get 通过 ID 查询。
func (r *Repo) Get(ctx context.Context, id string) (*Reservation, error) {
	row := r.pg.QueryRow(ctx, `
		SELECT id, member_id, store_id, slot, seats, status, created_at
		FROM reservations WHERE id=$1
	`, id)
	var res Reservation
	if err := row.Scan(&res.ID, &res.MemberID, &res.StoreID, &res.Slot, &res.Seats, &res.Status, &res.CreatedAt); err != nil {
		return nil, err
	}
	return &res, nil
}

// Ping 健康检查。
func (r *Repo) Ping(ctx context.Context) error {
	if err := r.pg.Ping(ctx); err != nil {
		return err
	}
	return r.cache.Ping(ctx).Err()
}
