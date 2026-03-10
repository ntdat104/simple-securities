package conversion

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

// 1. ExtractPDFTextFromBytes: Dùng cho dữ liệu từ user upload (data []byte)
func ExtractPDFTextFromBytes(data []byte) (string, error) {
	reader := bytes.NewReader(data)
	pdfReader, err := pdf.NewReader(reader, int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("failed to create PDF reader from bytes: %w", err)
	}

	return parsePDFContent(pdfReader)
}

// 2. ExtractPDFTextFromPath: Dùng cho file ở local workspace (path string)
func ExtractPDFTextFromPath(path string) (string, error) {
	file, pdfReader, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open PDF file at %s: %w", path, err)
	}
	defer file.Close()

	return parsePDFContent(pdfReader)
}

// Hàm bổ trợ để duyệt qua các trang và lấy nội dung văn bản
func parsePDFContent(pdfReader *pdf.Reader) (string, error) {
	var textBuilder strings.Builder
	numPages := pdfReader.NumPage()

	for i := 1; i <= numPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}

		// Sử dụng GetPlainText để lấy text đơn giản nhất
		text, err := page.GetPlainText(nil)
		if err != nil {
			// Nếu một trang lỗi, ta bỏ qua và tiếp tục các trang khác
			continue
		}

		if strings.TrimSpace(text) != "" {
			if textBuilder.Len() > 0 {
				textBuilder.WriteString("\n\n") // Khoảng cách giữa các trang
			}
			// Thêm đánh dấu trang nếu cần thiết
			textBuilder.WriteString(fmt.Sprintf("--- Page %d ---\n", i))
			textBuilder.WriteString(text)
		}
	}

	return textBuilder.String(), nil
}
