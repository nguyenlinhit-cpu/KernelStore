package handlers

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/KernelStore/backend-go/internal/dto"
	"github.com/KernelStore/backend-go/internal/httpx"
	"github.com/KernelStore/backend-go/internal/models"
	"github.com/KernelStore/backend-go/internal/repository"
	"github.com/KernelStore/backend-go/internal/services"
)

// ReviewsController: /api/reviews.
func (h *Handler) registerReviews(rt *httpx.Router) {
	rt.Handle("GET /api/reviews", httpx.Anonymous, h.listReviews)
	rt.Handle("POST /api/reviews", httpx.Authenticated, h.createReview)
}

func (h *Handler) listReviews(w http.ResponseWriter, r *http.Request) {
	q := httpx.NewQuery(r)
	productID := q.GUID("productId")
	if q.Failed(w) {
		return
	}
	if productID == uuid.Nil {
		dto.BadRequest(w, "Thiếu productId")
		return
	}
	reviews, err := repository.ListProductReviews(r.Context(), h.db, productID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	list, avg := reviewSummary(reviews)
	dto.OK(w, dto.ProductReviewsDto{Reviews: list, AverageRating: avg, ReviewCount: len(list)}, "OK")
}

// createReview: chỉ khi đã có đơn Delivered chứa sản phẩm; mỗi user một đánh giá / sản phẩm.
func (h *Handler) createReview(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateReviewRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	ctx, uid := r.Context(), userID(r)

	product, err := repository.FindProductRow(ctx, h.db, req.ProductID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if product == nil {
		dto.NotFound(w, "Không tìm thấy sản phẩm")
		return
	}
	received, err := repository.UserReceivedProduct(ctx, h.db, uid, req.ProductID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if !received {
		dto.Forbidden(w, "Bạn cần xác nhận đã nhận hàng trước khi đánh giá")
		return
	}
	if dup, err := repository.UserReviewedProduct(ctx, h.db, uid, req.ProductID); err != nil {
		serverError(w, r, err)
		return
	} else if dup {
		dto.BadRequest(w, "Bạn đã đánh giá sản phẩm này")
		return
	}

	rv := &models.Review{
		ID: services.NewUUID(), ProductID: req.ProductID, UserID: uid,
		Rating: req.Rating, Comment: strings.TrimSpace(req.Comment), CreatedAt: services.Now(),
	}
	if err := repository.InsertReview(ctx, h.db, rv); err != nil {
		serverError(w, r, err)
		return
	}
	user, err := repository.FindUserByID(ctx, h.db, uid)
	if err != nil {
		serverError(w, r, err)
		return
	}
	var fullName string
	if user != nil {
		fullName = user.FullName
	}
	dto.OK(w, dto.ReviewDto{ID: rv.ID, Rating: rv.Rating, Comment: rv.Comment,
		CreatedAt: rv.CreatedAt, UserID: uid, UserName: fullName}, "Đã gửi đánh giá")
}
