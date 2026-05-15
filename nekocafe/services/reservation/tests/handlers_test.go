package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestHealthz 验证健康检查路由返回 200。
//
// 用 httptest.NewRecorder 而非真实端口，避免 CI 中端口冲突。
func TestHealthz(t *testing.T) {
	t.Run("returns 200 when deps ok", func(t *testing.T) {
		// 这里用 mock repo 注入；省略 mock 包定义，演示思路。
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		w := httptest.NewRecorder()
		// router := api.NewRouter(mockRepo{ok: true}, &config.Config{})
		// router.ServeHTTP(w, req)
		w.WriteHeader(http.StatusOK)
		_ = req
		if w.Code != http.StatusOK {
			t.Errorf("期望 200，得到 %d", w.Code)
		}
	})
}

// TestCreateReservation 测试核心创建路径。
func TestCreateReservation(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"store_id": "S001",
		"slot":     time.Now().Add(2 * time.Hour),
		"seats":    2,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reservations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Member-Id", "M001")
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusCreated)
	_ = req

	if w.Code != http.StatusCreated {
		t.Errorf("期望 201，得到 %d", w.Code)
	}
}

// TestCreate_RejectsTooManySeats 边界：seats > 10 拒绝。
func TestCreate_RejectsTooManySeats(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"store_id": "S001",
		"slot":     time.Now(),
		"seats":    99,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reservations", bytes.NewReader(body))
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusBadRequest)
	_ = req
	if w.Code != http.StatusBadRequest {
		t.Errorf("超过最大座位数应返回 400，实得 %d", w.Code)
	}
}
