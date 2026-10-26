# DCA

> Code: `backend/internal/dca`. Giao diện: menu DCA.

Ladder trên một pair (spot, cross hoặc isolated margin). Mỗi **nấc** do người dùng đặt: mua khi giá ≤ X,
bán khi giá ≥ Y (0 = giữ, không tự bán), khối lượng N coin. Hệ thống chỉ canh giá 5 phút/lần và khớp lệnh
market.

| Trạng thái nấc | Ý nghĩa | Sửa được |
|---|---|---|
| Chờ mua (`OPENING`) | Chờ giá ≤ giá mua | Mọi trường; xoá được |
| Đang giữ (`OPEN`) | Đã mua | Chỉ giá bán |
| Đã bán (`CLOSED`) | Xong | Không |

Mỗi lượt kiểm tra: bán trước (giải phóng vốn), mua sau, rồi cập nhật lãi/lỗ chưa chốt. Một nấc lỗi không chặn
các nấc khác; lỗi được ghi ở nấc và cảnh báo Discord (giới hạn tần suất).

Khác bản Rails:

- Giá kích hoạt (`buyPrice`, `sellPrice`) tách khỏi giá khớp (`openPrice`, `closePrice`).
- Lãi/lỗ tính cả phí. Phí mua trả bằng coin làm giảm số coin giữ, và lệnh bán chỉ bán đúng số coin đang giữ
  (Rails bán đủ `amount` nên có thể thiếu số dư).
- Giới hạn vốn tuỳ chọn (Cấu hình DCA): tổng USDT trong các nấc đang giữ, trên mọi ladder. 0 = không giới hạn.
