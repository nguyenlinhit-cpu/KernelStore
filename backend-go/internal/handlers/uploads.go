package handlers

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/KernelStore/backend-go/internal/dto"
	"github.com/KernelStore/backend-go/internal/httpx"
	"github.com/KernelStore/backend-go/internal/services"
)

// Giới hạn [RequestSizeLimit(5 MB)] của UploadsController.
const maxUploadBytes = 5 << 20

// Đuôi file được nhận (quyết định loại file chọn được ở client).
var allowedImageExt = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".svg": true}

// UploadsController: /api/uploads.
func (h *Handler) registerUploads(rt *httpx.Router) {
	rt.Handle("POST /api/uploads/image", httpx.Authenticated, h.uploadImage)
}

func (h *Handler) uploadImage(w http.ResponseWriter, r *http.Request) {
	// IFormFile + [ApiController] ⇒ [FromForm]: body không phải form → 415.
	mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mt != "multipart/form-data" && mt != "application/x-www-form-urlencoded" {
		dto.Fail(w, http.StatusUnsupportedMediaType, "Content-Type phải là multipart/form-data")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			dto.Fail(w, http.StatusRequestEntityTooLarge, "File quá lớn (tối đa 5MB)")
			return
		}
		dto.BadRequest(w, "Dữ liệu không hợp lệ", "file: "+err.Error())
		return
	}

	fh := formFile(r, "file")
	if fh == nil || fh.Size == 0 {
		dto.BadRequest(w, "Chưa chọn file")
		return
	}
	if fh.Size > maxUploadBytes {
		dto.BadRequest(w, "File quá lớn (tối đa 5MB)")
		return
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if !allowedImageExt[ext] {
		dto.BadRequest(w, "Chỉ chấp nhận ảnh jpg, png hoặc svg")
		return
	}

	// Tên file = Guid dạng "N" (32 hex, không gạch) + đuôi.
	name := strings.ReplaceAll(services.NewUUID().String(), "-", "") + ext
	if err := saveUpload(fh, filepath.Join(h.uploadDir, name)); err != nil {
		serverError(w, r, err)
		return
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	dto.OK(w, map[string]string{"url": scheme + "://" + r.Host + "/uploads/" + name}, "Đã tải ảnh lên")
}

// formFile tìm file theo tên field, không phân biệt hoa thường như model binding ASP.NET.
func formFile(r *http.Request, name string) *multipart.FileHeader {
	if r.MultipartForm == nil {
		return nil
	}
	for k, files := range r.MultipartForm.File {
		if strings.EqualFold(k, name) && len(files) > 0 {
			return files[0]
		}
	}
	return nil
}

func saveUpload(fh *multipart.FileHeader, dst string) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	return out.Close()
}
