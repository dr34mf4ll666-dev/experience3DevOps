package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nekocafe/reservation/config"
	"github.com/nekocafe/reservation/repository"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
)

var tracer = otel.Tracer("reservation/api")

type Handlers struct {
	repo *repository.Repo
	cfg  *config.Config
}

type CreateRequest struct {
	StoreID string    `json:"store_id" binding:"required"`
	Slot    time.Time `json:"slot"     binding:"required"`
	Seats   int       `json:"seats"    binding:"required,min=1,max=10"`
}

func (h *Handlers) Create(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "reservations.create")
	defer span.End()

	memberID := c.GetHeader("X-Member-Id")
	if memberID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少 X-Member-Id"})
		return
	}

	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	span.SetAttributes(
		attribute.String("member.id", memberID),
		attribute.String("store.id", req.StoreID),
		attribute.Int("seats", req.Seats),
	)

	if err := h.checkMemberQuota(ctx, memberID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	res := &repository.Reservation{
		MemberID: memberID,
		StoreID:  req.StoreID,
		Slot:     req.Slot,
		Seats:    req.Seats,
	}
	if err := h.repo.Create(ctx, res); err != nil {
		span.RecordError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *Handlers) Get(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "reservations.get")
	defer span.End()
	id := c.Param("id")
	span.SetAttributes(attribute.String("reservation.id", id))

	res, err := h.repo.Get(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handlers) checkMemberQuota(ctx context.Context, memberID string) error {
	ctx, span := tracer.Start(ctx, "member.check_quota")
	defer span.End()

	url := fmt.Sprintf("%s/api/v1/members/%s/quota", h.cfg.MemberServiceURL, memberID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	// 注入 traceparent header
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// member 服务不可达时降级：允许继续（避免强依赖）
		span.RecordError(err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("会员不存在")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("会员校验失败: status=%d", resp.StatusCode)
	}

	var out struct {
		Remaining int `json:"remaining"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	span.SetAttributes(attribute.Int("member.remaining", out.Remaining))
	if out.Remaining <= 0 {
		return fmt.Errorf("本月预约额度已用尽")
	}
	return nil
}
